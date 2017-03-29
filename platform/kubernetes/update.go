package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/weaveworks/flux"
)

// UpdatePodController takes the body of a ReplicationController or Deployment
// resource definition (specified in YAML) and the name of the new image that
// should be put in the definition (in the format "repo.org/group/name:tag"). It
// returns a new resource definition body where all references to the old image
// have been replaced with the new one.
//
// This function has many additional requirements that are likely in flux. Read
// the source to learn about them.
func UpdatePodController(def []byte, newImageID flux.ImageID, trace io.Writer) ([]byte, error) {
	// Sanity check
	obj, err := definitionObj(def)
	if err != nil {
		return nil, err
	}
	switch obj.Kind {
	case "ReplicationController":
		return nil, ErrReplicationControllersDeprecated
	case "Deployment":
		break
	default:
		return nil, UpdateNotSupportedError(obj.Kind)
	}

	var buf bytes.Buffer
	err = tryUpdate(string(def), newImageID, trace, &buf)
	return buf.Bytes(), err
}

// Attempt to update an RC or Deployment config. This makes several assumptions
// that are justified only with the phrase "because that's how we do it",
// including:
//
//  * the file is a replication controller or deployment
//  * the update is from one tag of an image to another tag of the
//    same image; e.g., "weaveworks/helloworld:a00001" to
//    "weaveworks/helloworld:a00002"
//  * the container spec to update is the (first) one that uses the
//    same image name (e.g., weaveworks/helloworld)
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
//   ...                             # ) any number of equally-indented lines
//   name: helloworld-master-a000001 # ) can precede the name
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
func tryUpdate(def string, newImage flux.ImageID, trace io.Writer, out io.Writer) error {
	nameRE := multilineRE(
		`metadata:\s*`,
		`(?:  .*\n)*  name:\s*"?([\w-]+)"?\s*`,
	)
	matches := nameRE.FindStringSubmatch(def)
	if matches == nil || len(matches) < 2 {
		return fmt.Errorf("Could not find resource name")
	}
	oldDefName := matches[1]
	fmt.Fprintf(trace, "Found resource name %q in fragment:\n\n%s\n\n", oldDefName, matches[0])

	imageRE := multilineRE(
		`      containers:.*`,
		`(?:      .*\n)*(?:  ){3,4}- name:\s*"?([\w-]+)"?(?:\s.*)?`,
		`(?:  ){4,5}image:\s*"?(`+newImage.Repository()+`(:[\w][\w.-]{0,127})?)"?(\s.*)?`,
	)
	// tag part of regexp from
	// https://github.com/docker/distribution/blob/master/reference/regexp.go#L36

	matches = imageRE.FindStringSubmatch(def)
	if matches == nil || len(matches) < 3 {
		return fmt.Errorf("Could not find image name: %s", newImage.Repository())
	}
	containerName := matches[1]
	oldImage, err := flux.ParseImageID(matches[2])
	if err != nil {
		return err
	}
	fmt.Fprintf(trace, "Found container %q using image %v in fragment:\n\n%s\n\n", containerName, oldImage, matches[0])

	if oldImage.Repository() != newImage.Repository() {
		return fmt.Errorf(`expected existing image name and new image name to match, but %q != %q`, oldImage.Repository(), newImage.Repository())
	}

	// Now to replace bits. Specifically,
	// * the name, with a re-tagged name
	// * the image for the container
	// * the version label (in two places)
	//
	// Some values (most likely the version) will be interpreted as a
	// number if unquoted; while, on the other hand, it is apparently
	// not OK to quote things that don't look like numbers. So: we
	// extract values *without* quotes, and add them if necessary.

	newDefName := oldDefName
	_, _, oldImageTag := oldImage.Components()
	_, _, newImageTag := newImage.Components()
	if strings.HasSuffix(oldDefName, oldImageTag) {
		newDefName = oldDefName[:len(oldDefName)-len(oldImageTag)] + newImageTag
	}

	newDefName = maybeQuote(newDefName)
	newTag := maybeQuote(newImageTag)

	fmt.Fprintln(trace, "")
	fmt.Fprintln(trace, "Replacing ...")
	fmt.Fprintf(trace, "Resource name: %s -> %s\n", oldDefName, newDefName)
	fmt.Fprintf(trace, "Version in templates (and selector if present): %s -> %s\n", oldImageTag, newTag)
	fmt.Fprintf(trace, "Image in templates: %s -> %s\n", oldImage, newImage)
	fmt.Fprintln(trace, "")

	// The name we want is that under `metadata:`, which will be indented once
	replaceRCNameRE := regexp.MustCompile(`(?m:^(  name:\s*) (?:"?[\w-]+"?)(\s.*)$)`)
	withNewDefName := replaceRCNameRE.ReplaceAllString(def, fmt.Sprintf(`$1 %s$2`, newDefName))

	// Replacing labels: these are in two places, the container template and the selector
	replaceLabelsRE := multilineRE(
		`((?:  selector|      labels):.*)`,
		`((?:  ){2,4}name:.*)`,
		`((?:  ){2,4}version:\s*) (?:"?[-\w]+"?)(\s.*)`,
	)
	replaceLabels := fmt.Sprintf("$1\n$2\n$3 %s$4", newTag)
	withNewLabels := replaceLabelsRE.ReplaceAllString(withNewDefName, replaceLabels)

	replaceImageRE := multilineRE(
		`((?:  ){3,4}- name:\s*`+containerName+`)`,
		`((?:  ){4,5}image:\s*) .*`,
	)
	replaceImage := fmt.Sprintf("$1\n$2 %s$3", newImage.String())
	withNewImage := replaceImageRE.ReplaceAllString(withNewLabels, replaceImage)

	fmt.Fprint(out, withNewImage)
	return nil
}

func multilineRE(lines ...string) *regexp.Regexp {
	return regexp.MustCompile(`(?m:^` + strings.Join(lines, "\n") + `$)`)
}

var looksLikeNumber *regexp.Regexp = regexp.MustCompile("^(" + strings.Join([]string{
	`(-?[1-9](\.[0-9]*[1-9])?(e[-+][1-9][0-9]*)?)`,
	`(-?(0|[1-9][0-9]*))`,
	`(0|(\.inf)|(-\.inf)|(\.nan))`},
	"|") + ")$")

func maybeQuote(scalar string) string {
	if looksLikeNumber.MatchString(scalar) {
		return `"` + scalar + `"`
	}
	return scalar
}
