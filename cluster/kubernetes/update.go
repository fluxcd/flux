package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/image"
)

// updatePodController takes the body of a Deployment resource definition
// (specified in YAML) and the name of the new image that should be put in the
// definition (in the format "repo.org/group/name:tag"). It returns a new
// resource definition body where all references to the old image have been
// replaced with the new one.
//
// This function has many additional requirements that are likely in flux. Read
// the source to learn about them.
func updatePodController(def []byte, container string, newImageID image.Ref) ([]byte, error) {
	// Sanity check
	obj, err := definitionObj(def)
	if err != nil {
		return nil, err
	}

	_, supported := resourceKinds[strings.ToLower(obj.Kind)]

	// Lists are okay; we handle those downstream
	if obj.Kind != "List" && !supported {
		return nil, UpdateNotSupportedError(obj.Kind)
	}

	var buf bytes.Buffer
	err = tryUpdate(def, container, newImageID, &buf)
	return buf.Bytes(), err
}

func findContainersInList(l resource.List) []resource.Container {
	var containers []resource.Container

	for _, item := range l.Items {
		containers = append(containers, getContainersFromManifest(item)...)
	}
	return containers
}

func getContainersFromManifest(manifest resource.BaseObject) []resource.Container {
	return manifest.Spec.Template.Spec.Containers
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
func tryUpdate(def []byte, container string, newImage image.Ref, out io.Writer) error {
	manifest, err := parseManifest(def)

	if err != nil {
		return err
	}

	if manifest.Metadata.Name == "" && manifest.Kind != "List" {
		return fmt.Errorf("could not find resource name")
	}

	// Check if any containers need updating. As we go through, we calculate the
	// new manifest name, in case it includes the image tag (as in replication
	// controllers).
	newDefName := manifest.Metadata.Name
	matchingContainers := map[int]resource.Container{}
	var manifestContainers []resource.Container

	if manifest.Kind == "List" {
		list := resource.List{}
		err := yaml.Unmarshal(def, &list)

		if err != nil {
			return err
		}

		manifestContainers = findContainersInList(list)
	} else {
		manifestContainers = getContainersFromManifest(manifest)
	}

	for i, c := range manifestContainers {
		if c.Name != container {
			continue
		}
		currentImage, err := image.ParseRef(c.Image)
		if err != nil {
			return fmt.Errorf("could not parse image %s", c.Image)
		}
		if currentImage.CanonicalName() == newImage.CanonicalName() {
			matchingContainers[i] = c
		}
		_, _, oldImageTag := currentImage.Components()
		if strings.HasSuffix(manifest.Metadata.Name, oldImageTag) {
			newDefName = manifest.Metadata.Name[:len(manifest.Metadata.Name)-len(oldImageTag)] + newImage.Tag
		}
	}

	// Some values (most likely the version) will be interpreted as a
	// number if unquoted; while, on the other hand, it is apparently
	// not OK to quote things that don't look like numbers. So: we
	// extract values *without* quotes, and add them if necessary.
	newDefName = maybeQuote(newDefName)

	if len(matchingContainers) == 0 {
		return fmt.Errorf("could not find container using image: %s", newImage.Repository())
	}

	// Detect how indented the "containers" block is.
	newDef := string(def)
	matches := regexp.MustCompile(`( +)containers:.*`).FindStringSubmatch(newDef)
	if len(matches) != 2 {
		return fmt.Errorf("could not find container specs")
	}
	indent := matches[1]

	// Replace the container images
	// Parse out all the container blocks
	containersRE := regexp.MustCompile(`(?m:^` + indent + `containers:\s*(?:#.*)*$(?:\n(?:` + indent + `[-\s#].*)?)*)`)
	// Parse out an individual container blog
	containerRE := regexp.MustCompile(`(?m:` + indent + `-.*(?:\n(?:` + indent + `\s+.*)?)*)`)
	// Parse out the image ID
	imageRE := regexp.MustCompile(`(` + indent + `[-\s]\s*"?image"?:\s*)"?(?:[\w\.\-/:]+\s*?)*"?([\t\f #]+.*)?`)
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

	// The name we want is that under `metadata:`, which will *probably* be the first one
	replacedName := false
	replaceRCNameRE := regexp.MustCompile(`(\s+"?name"?:\s*)"?(?:[\w\.\-/:]+\s*?)"?([\t\f #]+.*)`)
	replaceRCNameRE.ReplaceAllStringFunc(newDef, func(found string) string {
		if replacedName {
			return found
		}
		replacedName = true
		return replaceRCNameRE.ReplaceAllString(found, fmt.Sprintf(`${1}%s${2}`, newDefName))
	})

	// Replacing labels: these are in two places, the container template and the selector
	// TODO: This doesn't handle # comments
	// TODO: This encodes an expectation of map keys being ordered (i.e. name *then* version)
	// TODO: This assumes that these are indented by exactly 2 spaces (which may not be true)
	replaceLabelsRE := multilineRE(
		`((?:  selector|      labels):.*)`,
		`((?:  ){2,4}name:.*)`,
		`((?:  ){2,4}version:\s*) (?:"?[-\w]+"?)(\s.*)`,
	)
	replaceLabels := fmt.Sprintf("$1\n$2\n$3 %s$4", maybeQuote(newImage.Tag))
	newDef = replaceLabelsRE.ReplaceAllString(newDef, replaceLabels)

	fmt.Fprint(out, newDef)
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
