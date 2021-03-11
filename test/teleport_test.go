package test

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	k8sYaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

type Node struct {
	Kind     string
	Metadata struct {
		Name string
	}
}

func teleportNodeServiceTest() {
	By("retrieving LoadBalancer IP address of teleport auth service")
	var addr string
	Eventually(func() error {
		stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "teleport", "get", "service", "teleport-auth",
			"--output=jsonpath={.status.loadBalancer.ingress[0].ip}")
		if err != nil {
			return fmt.Errorf("stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		ret := strings.TrimSpace(string(stdout))
		if len(ret) == 0 {
			return errors.New("teleport auth IP address is empty")
		}
		addr = ret
		return nil
	}).Should(Succeed())

	By("storing LoadBalancer IP address to etcd")
	ExecSafeAt(boot0, "env", "ETCDCTL_API=3", "etcdctl", "--cert=/etc/etcd/backup.crt", "--key=/etc/etcd/backup.key",
		"put", "/neco/teleport/auth-servers", `[\"`+addr+`:3025\"]`)

	By("starting teleport node services on boot servers")
	for _, h := range []string{boot0, boot1, boot2} {
		ExecSafeAt(h, "sudo", "neco", "teleport", "config")
		ExecSafeAt(h, "sudo", "systemctl", "start", "teleport-node.service")
	}
}

func teleportSSHConnectionTest() {
	// Run on boot1 because this test changes kubectl config and it causes failures of other tests running in parallel when those execute kubectl on boot0
	By("prepare .kube/config on boot1")
	_, stderr, err := ExecAt(boot1, "mkdir", "-p", "~/.kube")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	_, stderr, err = ExecAt(boot1, "ckecli", "kubernetes", "issue", ">", "~/.kube/config")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("adding proxy addr entry to /etc/hosts")
	stdout, stderr, err := ExecAt(boot1, "kubectl", "-n", "teleport", "get", "service", "teleport-proxy",
		"--output=jsonpath={.status.loadBalancer.ingress[0].ip}")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	addr := string(stdout)
	entry := fmt.Sprintf("%s teleport.gcp0.dev-ne.co", addr)
	_, stderr, err = ExecAt(boot1, "sudo", "sh", "-c", fmt.Sprintf(`'echo "%s" >> /etc/hosts'`, entry))
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("creating user")
	ExecAt(boot1, "kubectl", "-n", "teleport", "exec", "teleport-auth-0", "tctl", "users", "rm", "cybozu")
	stdout, stderr, err = ExecAt(boot1, "kubectl", "-n", "teleport", "exec", "teleport-auth-0", "tctl", "users", "add", "cybozu", "cybozu,root")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	tctlOutput := string(stdout)
	fmt.Println("output:")
	fmt.Println(tctlOutput)

	By("extracting invite token")
	/* target is b86d5b576174f7bbcb87d4905366aa9a in this example:
	User cybozu has been created but requires a password. Share this URL with the user to complete user setup, link is valid for 1h0m0s:
	https://teleport.gcp0.dev-ne.co:443/web/invite/b86d5b576174f7bbcb87d4905366aa9a

	NOTE: Make sure teleport.gcp0.dev-ne.co:443 points at a Teleport proxy which users can access.
	*/
	inviteURL, err := grepLine(tctlOutput, "https://")
	Expect(err).ShouldNot(HaveOccurred())
	slashSplit := strings.Split(inviteURL, "/")
	inviteToken := slashSplit[len(slashSplit)-1]
	Expect(inviteToken).NotTo(BeEmpty())
	fmt.Println("invite token: " + inviteToken)

	By("constructing payload")
	payload, err := json.Marshal(map[string]string{
		"token":               inviteToken,
		"password":            base64.StdEncoding.EncodeToString([]byte("dummypass")),
		"second_factor_token": "",
	})
	Expect(err).ShouldNot(HaveOccurred())
	fmt.Println("payload: " + string(payload))

	By("accessing invite URL")
	filename := "teleport_cookie.txt"
	_, stderr, err = ExecAt(boot1, "curl", "--fail", "--insecure", "-c", filename, inviteURL)
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	stdout, stderr, err = ExecAt(boot1, "cat", filename)
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	cookieFileContents := string(stdout)
	fmt.Println("cookie file:")
	fmt.Println(cookieFileContents)

	By("extracting CSRF token")
	/* target is c7c59fea8ec95e81c81b285e0070cb4791e04733fa4f22dffff5a25bb5b1c4f7 in this example:
	# Netscape HTTP Cookie File
	# https://curl.haxx.se/docs/http-cookies.html
	# This file was generated by libcurl! Edit at your own risk.

	#HttpOnly_teleport.gcp0.dev-ne.co	FALSE	/	TRUE	0	grv_csrf	c7c59fea8ec95e81c81b285e0070cb4791e04733fa4f22dffff5a25bb5b1c4f7

	*/
	csrfLine, err := grepLine(cookieFileContents, "#HttpOnly_")
	Expect(err).ShouldNot(HaveOccurred(), "output=%s", stdout)
	csrfLineFields := strings.Fields(csrfLine)
	csrfToken := csrfLineFields[len(csrfLineFields)-1]
	Expect(csrfToken).NotTo(BeEmpty())
	fmt.Printf("CSRF token: %s\n", csrfToken)

	By("updating password")
	_, stderr, err = ExecAt(boot1,
		"curl",
		"--fail", "--insecure",
		"-X", "PUT",
		"-b", filename,
		"-H", fmt.Sprintf(`'X-CSRF-Token: %s'`, csrfToken),
		"-H", `'Content-Type: application/json; charset=UTF-8'`,
		"-d", fmt.Sprintf(`'%s'`, string(payload)),
		"https://teleport.gcp0.dev-ne.co/v1/webapi/users/password/token",
	)
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("logging in using tsh command")
	Eventually(func() error {
		// Use ssh command and run tsh to input password using pty
		var cmd *exec.Cmd
		if placematMajorVersion == "1" {
			cmd = exec.Command("nsenter", "-n", "-t", operationPID, "ssh", "-oStrictHostKeyChecking=no", "-i", sshKeyFile,
				fmt.Sprintf("cybozu@%s", boot1), "-t", "tsh", "--insecure", "--proxy=teleport.gcp0.dev-ne.co:443", "--user=cybozu login")
		} else {
			cmd = exec.Command("ip", "netns", "exec", "operation", "ssh", "-oStrictHostKeyChecking=no", "-i", sshKeyFile,
				fmt.Sprintf("cybozu@%s", boot1), "-t", "tsh", "--insecure", "--proxy=teleport.gcp0.dev-ne.co:443", "--user=cybozu login")
		}
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return fmt.Errorf("pts.Start failed: %w", err)
		}
		defer ptmx.Close()
		_, err = ptmx.Write([]byte("dummypass\n"))
		if err != nil {
			return fmt.Errorf("ptmx.Write failed: %w", err)
		}
		go func() { io.Copy(os.Stdout, ptmx) }()
		return cmd.Wait()
	}).Should(Succeed())

	By("getting node resources with kubectl via teleport proxy")
	_, stderr, err = ExecAt(boot1, "kubectl", "get", "nodes")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("accessing boot servers using tsh command")
	for _, n := range []string{"boot-0", "boot-1", "boot-2"} {
		Eventually(func() error {
			_, stderr, err := ExecAt(boot1, "tsh", "--insecure", "--proxy=teleport.gcp0.dev-ne.co:443", "--user=cybozu", "ssh", "cybozu@gcp0-"+n, "date")
			if err != nil {
				return fmt.Errorf("tsh ssh failed for %s: %s", n, string(stderr))
			}
			return nil
		}).Should(Succeed())
	}

	By("confirming kubectl works in node Pod using tsh command")
	for _, n := range []string{"node-maneki-0", "node-maneki-1"} {
		Eventually(func() error {
			stdout, stderr, err := ExecAt(boot1, "tsh", "--insecure", "--proxy=teleport.gcp0.dev-ne.co:443", "--user=cybozu", "ssh", "cybozu@"+n, ". /etc/profile.d/update-necocli.sh && kubectl -v5 -n maneki get pod")
			if err != nil {
				return fmt.Errorf("tsh ssh failed for %s: stdout=%s, stderr=%s", n, string(stdout), string(stderr))
			}
			return nil
		}).Should(Succeed())
	}

	By("logout tsh")
	_, stderr, err = ExecAt(boot1, "tsh", "--insecure", "--proxy=teleport.gcp0.dev-ne.co:443", "--user=cybozu", "logout")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("clearing kubectl config")
	_, stderr, err = ExecAt(boot1, "ckecli", "kubernetes", "issue", ">", "~/.kube/config")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("clearing teleport_cookie.txt")
	_, stderr, err = ExecAt(boot1, "rm", filename)
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)

	By("clearing /etc/hosts")
	_, stderr, err = ExecAt(boot1, "sudo", "sed", "-i", "-e", "/teleport.gcp0.dev-ne.co/d", "/etc/hosts")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
}

func teleportAuthTest() {
	By("getting the node list before recreating the teleport-auth pod")
	stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "teleport", "exec", "teleport-auth-0", "tctl", "get", "nodes")
	Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	beforeNodes := decodeNodes(stdout)

	By("recreating the teleport-auth pod")
	ExecSafeAt(boot0, "kubectl", "-n", "teleport", "delete", "pod", "teleport-auth-0")
	Eventually(func() error {
		stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "teleport", "exec", "teleport-auth-0", "tctl", "status")
		if err != nil {
			return fmt.Errorf("stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		return nil
	}).Should(Succeed())

	By("comparing the current node list with the obtained before")
	Eventually(func() error {
		stdout, stderr, err = ExecAt(boot0, "kubectl", "-n", "teleport", "exec", "teleport-auth-0", "tctl", "get", "nodes")
		if err != nil {
			return fmt.Errorf("stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		afterNodes := decodeNodes(stdout)
		if !cmp.Equal(afterNodes, beforeNodes) {
			return fmt.Errorf("before: %v, after: %v", beforeNodes, afterNodes)
		}
		return nil
	}).Should(Succeed())
}

func teleportApplicationTest() {
	// This test requires CNAME record "teleport.gcp0.dev-ne.co : teleport-proxy.teleport.svc".
	By("getting the application names")
	stdout, _, err := kustomizeBuild("../teleport/base/apps")
	Expect(err).ShouldNot(HaveOccurred())
	var appNames []string
	y := k8sYaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(stdout)))
	for {
		data, err := y.Read()
		if err == io.EOF {
			break
		}
		Expect(err).ShouldNot(HaveOccurred())

		var deploy appsv1.Deployment
		err = yaml.Unmarshal(data, &deploy)
		if err != nil {
			continue
		}

		var name string
		for _, a := range deploy.Spec.Template.Spec.Containers[0].Args {
			if !strings.HasPrefix(a, "--app-name=") {
				continue
			}
			name = strings.Split(a, "=")[1]
		}
		Expect(name).ShouldNot(BeEmpty())
		appNames = append(appNames, name)
	}
	fmt.Printf("Found applications in manifests: %+v\n", appNames)

	By("checking applications are correctly deployed")
	Eventually(func() error {
		for _, n := range appNames {
			query := fmt.Sprintf("'.[].spec.apps[].name | select(. == \"%s\")'", n)
			stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "teleport", "exec", "-it", "teleport-auth-0", "--", "tctl", "apps", "ls", "--format=json", "--", "|", "jq", "-r", query)
			if err != nil {
				return fmt.Errorf("stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			if string(stdout) != n+"\n" {
				return fmt.Errorf("app %s mismatch: actual = %s", n, stdout)
			}
		}
		return nil
	}).Should(Succeed())
}

func decodeNodes(input []byte) []Node {
	r := bytes.NewReader(input)
	y := k8sYaml.NewYAMLReader(bufio.NewReader(r))

	var nodes []Node
	for {
		data, err := y.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil
		}

		var node Node
		err = yaml.Unmarshal(data, &node)
		if err != nil {
			return nil
		}
		nodes = append(nodes, node)
	}

	return nodes
}

func grepLine(input string, prefix string) (string, error) {
	reader := bufio.NewReader(strings.NewReader(input))

	for {
		line, isPrefix, err := reader.ReadLine()
		if err == io.EOF {
			return "", errors.New("no match line")
		}
		if isPrefix {
			return "", errors.New("too long line")
		}

		if !strings.HasPrefix(string(line), prefix) {
			continue
		}

		return string(line), nil
	}
}

func testTeleport() {
	It("should deploy teleport services", func() {
		By("teleportNodeServiceTest", teleportNodeServiceTest)
		By("teleportSSHConnectionTest", teleportSSHConnectionTest)
		By("teleportAuthTest", teleportAuthTest)
		By("teleportApplicationTest", teleportApplicationTest)
	})
}
