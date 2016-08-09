package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/weaveworks/fluxy/registry"
)

// UpdateReplicationController takes the body of a ReplicationController
// resource definition (specified in YAML) and the name of the new image that
// should be put in the definition (in the format "repo.org/group/name:tag"). It
// returns a new resource definition body where all references to the old image
// have been replaced with the new one.
//
// This function has many additional requirements that are likely in flux. Read
// the source to learn about them.
func UpdateReplicationController(def []byte, newImageName string, trace io.Writer) ([]byte, error) {
	var buf bytes.Buffer
	err := tryUpdateRC(string(def), newImageName, trace, &buf)
	return buf.Bytes(), err
}

// Attempt to update an RC config. This makes several assumptions
// that are justified only with the phrase "because that's how we do
// it", including:
//  * the file is a replication controller
//  * the update is from one tag of an image to another tag of the
//  same image; e.g., "weaveworks/helloworld:a00001" to
//  "weaveworks/helloworld:a00002"
//  * the container spec to update is the (first) one that uses the
//  same image name (e.g., weaveworks/helloworld)
//  * the name of the controller is updated to reflect the new tag
//  * there's a label which must be updated in both the pod spec and the selector
//  * the file uses canonical YAML syntax, that is, one line per item
//  * ... other assumptions as encoded in the regular expressions used
//
// Here's an example of the assumed structure:
//
// ```
// apiVersion: v1
// kind: ReplicationController # not presently checked
// metadata:                         # )
//   name: helloworld-master-a000001 # ) this structure, and naming scheme, are assumed
// spec:
//   replicas: 2
//   selector:                 # )
//     name: helloworld        # ) this use of labels is assumed
//     version: master-a000001 # )
//   template:
//     metadata:
//       labels:                   # )
//         name: helloworld        # ) this structure is assumed, as for the selector
//         version: master-a000001 # )
//     spec:
//       containers:
//       # extra container specs are allowed here ...
//       - name: helloworld                                    # )
//         image: quay.io/weaveworks/helloworld:master-a000001 # ) these must be together
//         args:
//         - -msg=Ahoy
//         ports:
//         - containerPort: 80
// ```
func tryUpdateRC(rc, newImageStr string, trace io.Writer, out io.Writer) error {
	newImage := registry.ParseImage(newImageStr)

	nameRE := multilineRE(
		`metadata:\s*`,
		`  name:\s*"?([\w-]+)"?\s*`,
	)
	matches := nameRE.FindStringSubmatch(rc)
	if matches == nil || len(matches) < 2 {
		return fmt.Errorf("Could not find replication controller name")
	}
	oldRCName := matches[1]
	fmt.Fprintf(trace, "Found replication controller name %q in fragment:\n\n%s\n\n", oldRCName, matches[0])

	imageRE := multilineRE(
		`      containers:.*`,
		`(?:      .*\n)*      - name:\s*"?([\w-]+)"?(?:\s.*)?`,
		`        image:\s*"?(`+newImage.Repository()+`:[\w-]+)"?(\s.*)?`,
	)

	matches = imageRE.FindStringSubmatch(rc)
	if matches == nil || len(matches) < 3 {
		return fmt.Errorf("Could not find image name")
	}
	containerName := matches[1]
	oldImage := registry.ParseImage(matches[2])
	fmt.Fprintf(trace, "Found container %q using image %v in fragment:\n\n%s\n\n", containerName, oldImage, matches[0])

	if oldImage.Repository() != newImage.Repository() {
		return fmt.Errorf(`expected existing image name and new image name to match, but %q != %q`, oldImage.Repository(), newImage.Repository())
	}

	// Now to replace bits. Specifically,
	// * the name, with a re-tagged name
	// * the image for the container
	// * the version label (in two places)

	newRCName := oldRCName
	if strings.HasSuffix(oldRCName, oldImage.Tag) {
		newRCName = oldRCName[:len(oldRCName)-len(oldImage.Tag)] + newImage.Tag
	}

	fmt.Fprintln(trace, "")
	fmt.Fprintln(trace, "Replacing ...")
	fmt.Fprintf(trace, "Replication controller name: %q -> %q\n", oldRCName, newRCName)
	fmt.Fprintf(trace, "Version in container %q and selector: %q -> %q\n", containerName, oldImage.Tag, newImage.Tag)
	fmt.Fprintf(trace, "Image for container %q: %q -> %q\n", containerName, oldImage, newImage)
	fmt.Fprintln(trace, "")

	// The name we want is that under metadata:, which will be indented once
	replaceRCNameRE := regexp.MustCompile(`(?m:^(  name:\s*)(?:"?[\w-]+"?)(\s.*)$)`)
	withNewRCName := replaceRCNameRE.ReplaceAllString(rc, fmt.Sprintf(`$1%q$2`, newRCName))

	// Replacing labels: these are in two places, the container template and the selector
	replaceLabelsRE := multilineRE(
		`((?:  selector|      labels):.*)`,
		`((?:  ){2,4}name:.*)`,
		`((?:  ){2,4}version:\s*)(?:"?[-\w]+"?)(\s.*)`,
	)
	replaceLabels := fmt.Sprintf("$1\n$2\n$3%q$4", newImage.Tag)
	withNewLabels := replaceLabelsRE.ReplaceAllString(withNewRCName, replaceLabels)

	replaceImageRE := multilineRE(
		`(      - name:\s*`+containerName+`)`,
		`(        image:\s*).*`,
	)
	replaceImage := fmt.Sprintf("$1\n$2%q$3", newImage.String())
	withNewImage := replaceImageRE.ReplaceAllString(withNewLabels, replaceImage)

	fmt.Fprint(out, withNewImage)
	return nil
}

func multilineRE(lines ...string) *regexp.Regexp {
	return regexp.MustCompile(`(?m:^` + strings.Join(lines, "\n") + `$)`)
}
