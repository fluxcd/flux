package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// updatePodController takes the body of a resource definition
// (specified in YAML), the ID of a particular resource and container
// therein, and the name of the new image that should be put in the
// definition (in the format "repo.org/group/name:tag") for that
// resource and container. It returns a new resource definition body
// where all references to the old image have been replaced with the
// new one.
//
// This function has some requirements of the YAML structure. Read the
// source and comments below to learn about them.
func updatePodController(original io.Reader, resourceID flux.ResourceID, container string, newImageID image.Ref) (io.Reader, error) {
	copy := &bytes.Buffer{}
	tee := io.TeeReader(original, copy)
	// Sanity check
	obj, err := parseObj(tee)
	if err != nil {
		return nil, err
	}

	if _, ok := resourceKinds[strings.ToLower(obj.Kind)]; !ok {
		return nil, UpdateNotSupportedError(obj.Kind)
	}

	var buf bytes.Buffer
	err = tryUpdate(copy, resourceID, container, newImageID, &buf)
	return &buf, err
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
// kind: Deployment # not presently checked
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
func tryUpdate(def io.Reader, id flux.ResourceID, container string, newImage image.Ref, out io.Writer) error {
	copy := &bytes.Buffer{}
	tee := io.TeeReader(def, copy)
	containers, err := extractContainers(tee, id)

	matchingContainers := map[int]resource.Container{}
	for i, c := range containers {
		if c.Name != container {
			continue
		}
		currentImage := c.Image
		if err != nil {
			return fmt.Errorf("could not parse image %s", c.Image)
		}
		if currentImage.CanonicalName() == newImage.CanonicalName() {
			matchingContainers[i] = c
		}
	}

	if len(matchingContainers) == 0 {
		return fmt.Errorf("could not find container using image: %s", newImage.Repository())
	}

	// Detect how indented the "containers" block is.
	// TODO: delete all regular expressions which are used to modify YAML.
	// See #1019. Modifying this is not recommended.
	newDef := string(copy.String())
	matches := regexp.MustCompile(`( +)containers:.*`).FindStringSubmatch(newDef)
	if len(matches) != 2 {
		return fmt.Errorf("could not find container specs")
	}
	indent := matches[1]

	// TODO: delete all regular expressions which are used to modify YAML.
	// See #1019. Modifying this is not recommended.
	optq := `["']?` // An optional single or double quote
	// Replace the container images
	// Parse out all the container blocks
	containersRE := regexp.MustCompile(`(?m:^` + indent + `containers:\s*(?:#.*)*$(?:\n(?:` + indent + `[-\s#].*)?)*)`)
	// Parse out an individual container blog
	containerRE := regexp.MustCompile(`(?m:` + indent + `-.*(?:\n(?:` + indent + `\s+.*)?)*)`)
	// Parse out the image ID
	imageRE := regexp.MustCompile(`(` + indent + `[-\s]\s*` + optq + `image` + optq + `:\s*)` + optq + `(?:[\w\.\-/:]+\s*?)*` + optq + `([\t\f #]+.*)?`)
	imageReplacement := fmt.Sprintf("${1}%s${2}", maybeQuote(newImage.String()))
	// Find the block of container specs
	newDef = containersRE.ReplaceAllStringFunc(newDef, func(containers string) string {
		i := 0
		// Find each container spec
		return containerRE.ReplaceAllStringFunc(containers, func(spec string) string {
			if _, ok := matchingContainers[i]; ok {
				// container matches, let's replace the image
				spec = imageRE.ReplaceAllString(spec, imageReplacement)
				delete(matchingContainers, i)
			}
			i++
			return spec
		})
	})

	if len(matchingContainers) > 0 {
		missed := []string{}
		for _, c := range matchingContainers {
			missed = append(missed, c.Name)
		}
		return fmt.Errorf("did not update expected containers: %s", strings.Join(missed, ", "))
	}

	// TODO: delete all regular expressions which are used to modify YAML.
	// See #1019. Modifying this is not recommended.
	// Replacing labels: these are in two places, the container template and the selector
	// TODO: This doesn't handle # comments
	// TODO: This encodes an expectation of map keys being ordered (i.e. name *then* version)
	// TODO: This assumes that these are indented by exactly 2 spaces (which may not be true)
	replaceLabelsRE := multilineRE(
		`((?:  selector|      labels):.*)`,
		`((?:  ){2,4}name:.*)`,
		`((?:  ){2,4}version:\s*) (?:`+optq+`[-\w]+`+optq+`)(\s.*)`,
	)
	replaceLabels := fmt.Sprintf("$1\n$2\n$3 %s$4", maybeQuote(newImage.Tag))
	newDef = replaceLabelsRE.ReplaceAllString(newDef, replaceLabels)

	fmt.Fprint(out, newDef)
	return nil
}

func multilineRE(lines ...string) *regexp.Regexp {
	return regexp.MustCompile(`(?m:^` + strings.Join(lines, "\n") + `$)`)
}

// TODO: delete all regular expressions which are used to modify YAML.
// See #1019. Modifying this is not recommended.
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
