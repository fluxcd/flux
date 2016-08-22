package kubernetes

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestOneResource(t *testing.T) {
	input := `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: fluxy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: fluxy
    spec:
      volumes:
      - name: key
        secret:
          secretName: fluxy-repo-key
      containers:
      - name: fluxy
        image: weaveworks/fluxy:master-a000001
        imagePullPolicy: Never # must build manually
        ports:
        - containerPort: 3030
        volumeMounts:
        - name: key
          mountPath: /var/run/secrets/fluxy/key
          readOnly: true
        args:
        - /home/flux/fluxd
`
	expected := []string{
		`@@ -5 +5 @@`,
		`-  name: fluxy`,
		`+  name: "fluxy"`,
		`@@ -19 +19 @@`,
		`-        image: weaveworks/fluxy:master-a000001`,
		`+        image: "weaveworks/fluxy:master-a000002"`,
	}
	testByDiff(t, input, "weaveworks/fluxy:master-a000002", expected)
}

func TestOneOfSomeResource(t *testing.T) {
	input := `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: fluxy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: fluxy
    spec:
      volumes:
      - name: key
        secret:
          secretName: fluxy-repo-key
      containers:
      - name: fluxy
        image: weaveworks/fluxy:master-a000001
        imagePullPolicy: Never # must build manually
        ports:
        - containerPort: 3030
        volumeMounts:
        - name: key
          mountPath: /var/run/secrets/fluxy/key
          readOnly: true
        args:
        - /home/flux/fluxd
---
apiVersion: v1
kind: Service
metadata:
  name: fluxy
spec:
  ports:
    - port: 3030
  selector:
    name: fluxy
`
	expected := []string{
		`@@ -5 +5 @@`,
		`-  name: fluxy`,
		`+  name: "fluxy"`,
		`@@ -19 +19 @@`,
		`-        image: weaveworks/fluxy:master-a000001`,
		`+        image: "weaveworks/fluxy:master-a000002"`,
	}
	testByDiff(t, input, "weaveworks/fluxy:master-a000002", expected)
}

// ---

func testByDiff(t *testing.T, original, newImage string, expectedDiff []string) {
	out, err := UpdatePodController([]byte(original), newImage, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	diff := diffBytes([]byte(original), out)

	if !diffCompare(expectedDiff, diff) {
		t.Fatal(diffReport(expectedDiff, diff))
	}
}

func diffBytes(a []byte, b []byte) []string {
	fileA, _ := ioutil.TempFile("", "fluxy-test")
	fileB, _ := ioutil.TempFile("", "fluxy-test")
	fileA.Write(a)
	fileA.Close()
	fileB.Write(b)
	fileB.Close()
	fmt.Fprintf(os.Stderr, "%s vs %s\n", fileA.Name(), fileB.Name())
	diff := exec.Command("diff", "-U", "0", fileA.Name(), fileB.Name())
	out, _ := diff.Output()
	lines := strings.Split(string(out), "\n")
	return lines[2 : len(lines)-1] // skip file headers, and trailing newline
}

func diffCompare(as []string, bs []string) bool {
	if len(as) != len(bs) {
		fmt.Fprintf(os.Stderr, "len(as) = %d, len(bs) = %d", len(as), len(bs))
		return false
	}
	for i, _ := range as {
		if as[i] != bs[i] {
			fmt.Fprintf(os.Stderr, "as[%d] = %q\nbs[%d] = %q", i, as[i], i, bs[i])
			return false
		}
	}
	return true
}

func diffReport(expected, actual []string) string {
	return "Expected:\n" + strings.Join(expected, "\n") + "\n\n" +
		"Got:\n" + strings.Join(actual, "\n")
}
