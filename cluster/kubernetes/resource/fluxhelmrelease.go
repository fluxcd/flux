package resource

import (
	"fmt"
	"io"

	"github.com/weaveworks/flux"
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
	apiv1 "k8s.io/api/core/v1"
)

type FluxHelmRelease struct {
	baseObject
	Spec ifv1.FluxHelmReleaseSpec
}

func (fhr FluxHelmRelease) Containers() []resource.Container {
	fmt.Println("### FHR CONTAINERS (calling fhr.Containers())")
	containers, err := fhr.createFluxFHRContainers()
	if err != nil {
		// log ?
	}
	return containers
}

// CreateK8sContainers creates a list of k8s containers as
func CreateK8sFHRContainers(spec ifv1.FluxHelmReleaseSpec) []apiv1.Container {
	fmt.Println("\n+++ CreateK8sFHRContainers +++\n")

	values := spec.Values

	containers := []apiv1.Container{}

	fmt.Printf("\t\tvalues = %+v\n", values)
	fmt.Printf("\t\tlen(values) = %d\n", len(values))

	if len(values) == 0 {
		return containers
	}

	imgInfo, ok := values["image"]

	// image info appears on the top level, so is associated directly with the chart
	if ok {
		imgInfoStr, ok := imgInfo.(string)
		if !ok {
			return containers
		}

		cont := apiv1.Container{Name: spec.ChartGitPath, Image: imgInfoStr}
		fmt.Printf("\t\t+++ containers : %+v\n\n", cont)

		containers = append(containers, cont)

		return containers
	}

	return []apiv1.Container{}
}

func TryFHRUpdate(def []byte, resourceID flux.ResourceID, container string, newImage image.Ref, out io.Writer) error {
	fmt.Println("FAKE Updating image tag info for FHR special")
	fmt.Println("=========================================")
	fmt.Println("\t\t*** in tryFHRUpdate")
	fmt.Printf("\t\t*** container: %s\n", container)
	fmt.Printf("\t\t*** newImage: %+v\n", newImage)

	fmt.Println("Updating image tag info for FHR special")
	fmt.Println("=========================================")

	//--------------------------------------
	// manifest, err := parseManifest(def)
	// if err != nil {
	// 	return err
	// }
	// if manifest.Metadata.Name == "" {
	// 	return fmt.Errorf("could not find resource name")
	// }

	// // Check if any containers need updating. As we go through, we calculate the
	// // new manifest name, in case it includes the image tag (as in replication
	// // controllers).
	// newDefName := manifest.Metadata.Name
	// matchingContainers := map[int]Container{}
	// for i, c := range append(manifest.Spec.Template.Spec.Containers, manifest.Spec.JobTemplate.Spec.Template.Spec.Containers...) {
	// 	if c.Name != container {
	// 		continue
	// 	}
	// 	currentImage, err := image.ParseRef(c.Image)
	// 	if err != nil {
	// 		return fmt.Errorf("could not parse image %s", c.Image)
	// 	}
	// 	if currentImage.CanonicalName() == newImage.CanonicalName() {
	// 		matchingContainers[i] = c
	// 	}
	// 	_, _, oldImageTag := currentImage.Components()
	// 	if strings.HasSuffix(manifest.Metadata.Name, oldImageTag) {
	// 		newDefName = manifest.Metadata.Name[:len(manifest.Metadata.Name)-len(oldImageTag)] + newImage.Tag
	// 	}
	// }

	// // Some values (most likely the version) will be interpreted as a
	// // number if unquoted; while, on the other hand, it is apparently
	// // not OK to quote things that don't look like numbers. So: we
	// // extract values *without* quotes, and add them if necessary.
	// newDefName = maybeQuote(newDefName)

	// if len(matchingContainers) == 0 {
	// 	return fmt.Errorf("could not find container using image: %s", newImage.Repository())
	// }

	// // Detect how indented the "containers" block is.
	// // TODO: delete all regular expressions which are used to modify YAML.
	// // See #1019. Modifying this is not recommended.
	// newDef := string(def)
	// matches := regexp.MustCompile(`( +)containers:.*`).FindStringSubmatch(newDef)
	// if len(matches) != 2 {
	// 	return fmt.Errorf("could not find container specs")
	// }
	// indent := matches[1]

	// // TODO: delete all regular expressions which are used to modify YAML.
	// // See #1019. Modifying this is not recommended.
	// optq := `["']?` // An optional single or double quote
	// // Replace the container images
	// // Parse out all the container blocks
	// containersRE := regexp.MustCompile(`(?m:^` + indent + `containers:\s*(?:#.*)*$(?:\n(?:` + indent + `[-\s#].*)?)*)`)
	// // Parse out an individual container blog
	// containerRE := regexp.MustCompile(`(?m:` + indent + `-.*(?:\n(?:` + indent + `\s+.*)?)*)`)
	// // Parse out the image ID
	// imageRE := regexp.MustCompile(`(` + indent + `[-\s]\s*` + optq + `image` + optq + `:\s*)` + optq + `(?:[\w\.\-/:]+\s*?)*` + optq + `([\t\f #]+.*)?`)
	// imageReplacement := fmt.Sprintf("${1}%s${2}", maybeQuote(newImage.String()))
	// // Find the block of container specs
	// newDef = containersRE.ReplaceAllStringFunc(newDef, func(containers string) string {
	// 	i := 0
	// 	// Find each container spec
	// 	return containerRE.ReplaceAllStringFunc(containers, func(spec string) string {
	// 		if _, ok := matchingContainers[i]; ok {
	// 			// container matches, let's replace the image
	// 			spec = imageRE.ReplaceAllString(spec, imageReplacement)
	// 			delete(matchingContainers, i)
	// 		}
	// 		i++
	// 		return spec
	// 	})
	// })

	// if len(matchingContainers) > 0 {
	// 	missed := []string{}
	// 	for _, c := range matchingContainers {
	// 		missed = append(missed, c.Name)
	// 	}
	// 	return fmt.Errorf("did not update expected containers: %s", strings.Join(missed, ", "))
	// }

	// // TODO: delete all regular expressions which are used to modify YAML.
	// // See #1019. Modifying this is not recommended.
	// // The name we want is that under `metadata:`, which will *probably* be the first one
	// replacedName := false
	// replaceRCNameRE := regexp.MustCompile(`(\s+` + optq + `name` + optq + `:\s*)` + optq + `(?:[\w\.\-/:]+\s*?)` + optq + `([\t\f #]+.*)`)
	// replaceRCNameRE.ReplaceAllStringFunc(newDef, func(found string) string {
	// 	if replacedName {
	// 		return found
	// 	}
	// 	replacedName = true
	// 	return replaceRCNameRE.ReplaceAllString(found, fmt.Sprintf(`${1}%s${2}`, newDefName))
	// })

	// // TODO: delete all regular expressions which are used to modify YAML.
	// // See #1019. Modifying this is not recommended.
	// // Replacing labels: these are in two places, the container template and the selector
	// // TODO: This doesn't handle # comments
	// // TODO: This encodes an expectation of map keys being ordered (i.e. name *then* version)
	// // TODO: This assumes that these are indented by exactly 2 spaces (which may not be true)
	// replaceLabelsRE := multilineRE(
	// 	`((?:  selector|      labels):.*)`,
	// 	`((?:  ){2,4}name:.*)`,
	// 	`((?:  ){2,4}version:\s*) (?:`+optq+`[-\w]+`+optq+`)(\s.*)`,
	// )
	// replaceLabels := fmt.Sprintf("$1\n$2\n$3 %s$4", maybeQuote(newImage.Tag))
	// newDef = replaceLabelsRE.ReplaceAllString(newDef, replaceLabels)

	// fmt.Fprint(out, newDef)
	// return nil
	//--------------------------------------
	return nil
}

// assumes only one image in the Spec.Values
func (fhr FluxHelmRelease) createFluxFHRContainers() ([]resource.Container, error) {
	fmt.Println("\n+++ createFluxFHRContainers +++\n")

	fmt.Printf("\t\tSPEC: %+v\n", fhr.Spec)

	values := fhr.Spec.Values
	containers := []resource.Container{}

	fmt.Printf("\t\tvalues for chart %s = %+v\n", fhr.Spec.ChartGitPath, values)
	fmt.Printf("\t\tlen(values) = %d\n", len(values))

	if len(values) == 0 {
		return containers, nil
	}

	imgInfo, ok := values["image"]

	// image info appears on the top level, so is associated directly with the chart
	if ok {
		imgInfoStr := imgInfo.(string)

		fmt.Printf("\t\t+++ imgInfo=%s\n", imgInfoStr)

		imageRef, err := image.ParseRef(imgInfoStr)
		fmt.Printf("\t\t+++ imageRef=%s\n", imageRef)
		fmt.Printf("\t\t+++ err = %v\n", err)

		if err != nil {
			return containers, err
		}
		containers = append(containers, resource.Container{Name: fhr.Spec.ChartGitPath, Image: imageRef})
		fmt.Printf("\t\t+++++ containers for chart %s: %+v\n\n", fhr.Spec.ChartGitPath, containers[0])

		return containers, nil
	}

	fmt.Println("\t\tvalues[image] not a string")
	fmt.Println("\n+++ END createFluxFHRContainers +++\n")

	return []resource.Container{}, nil
}

// func processImageInfo(values map[string]interface{}, value interface{}) (image.Ref, error) {
// 	var ref image.Ref
// 	var err error

// 	switch value.(type) {
// 	case string:
// 		val := value.(string)
// 		ref, err = processImageString(values, val)
// 		if err != nil {
// 			return image.Ref{}, err
// 		}
// 		return ref, nil

// 	case map[string]string:
// 		// image:
// 		// 			registry: docker.io   (sometimes missing)
// 		// 			repository: bitnami/mariadb
// 		// 			tag: 10.1.32					(sometimes version)
// 		val := value.(map[string]string)
// 		ref, err = processImageMap(val)
// 		if err != nil {
// 			return image.Ref{}, err
// 		}
// 		return ref, nil

// 	default:
// 		return image.Ref{}, image.ErrMalformedImageID
// 	}
// }

// func findImage(spec ifv1.FluxHelmReleaseSpec, param string, value interface{}) (string, image.Ref, error) {
// 	var ref image.Ref
// 	var err error
// 	values := spec.Values

// 	if param == "image" {
// 		switch value.(type) {
// 		case string:
// 			val := value.(string)
// 			ref, err = processImageString(values, val)
// 			if err != nil {
// 				return "", image.Ref{}, err
// 			}
// 			return spec.ChartGitPath, ref, nil

// 		case map[string]string:
// 			// image:
// 			// 			registry: docker.io   (sometimes missing)
// 			// 			repository: bitnami/mariadb
// 			// 			tag: 10.1.32					(sometimes version)
// 			val := value.(map[string]string)

// 			ref, err = processImageMap(val)
// 			if err != nil {
// 				return "", image.Ref{}, err
// 			}
// 			return spec.ChartGitPath, ref, nil

// 		// ???
// 		default:
// 			return "", image.Ref{}, image.ErrMalformedImageID
// 		}
// 	}

// 	switch value.(type) {
// 	case map[string]interface{}:
// 		// image information is nested ---------------------------------------------------
// 		// 		controller:
// 		// 			image:
// 		// 				repository: quay.io/kubernetes-ingress-controller/nginx-ingress-controller
// 		// 				tag: "0.12.0"

// 		// 		jupyter:
// 		// 			image:
// 		// 				repository: "daskdev/dask-notebook"
// 		// 				tag: "0.17.1"

// 		// 		zeppelin:
// 		// 			image: dylanmei/zeppelin:0.7.2

// 		// 		artifactory:
// 		//   		name: artifactory
// 		//  	  replicaCount: 1
// 		//  		image:
// 		//   		  repository: "docker.bintray.io/jfrog/artifactory-pro"
// 		//  		  version: 5.9.1
// 		//   		  pullPolicy: IfNotPresent
// 		val := value.(map[string]interface{})

// 		var cName string
// 		//var ok bool
// 		if cn, ok := val["name"]; !ok {
// 			cName = cn.(string)
// 		}

// 		refP, err := processMaybeImageMap(val)
// 		if err != nil {
// 			return "", image.Ref{}, err
// 		}
// 		return cName, *refP, nil

// 	default:
// 		return "", image.Ref{}, nil
// 	}
// }

// func processImageString(values chartutil.Values, val string) (image.Ref, error) {
// 	if t, ok := values["imageTag"]; ok {
// 		val = fmt.Sprintf("%s:%s", val, t)
// 	} else if t, ok := values["tag"]; ok {
// 		val = fmt.Sprintf("%s:%s", val, t)
// 	}
// 	ref, err := image.ParseRef(val)
// 	if err != nil {
// 		return image.Ref{}, err
// 	}
// 	// returning chart to be the container name
// 	return ref, nil
// }

// func processImageMap(val map[string]string) (image.Ref, error) {
// 	var ref image.Ref
// 	var err error

// 	i, iOk := val["repository"]
// 	if !iOk {
// 		return image.Ref{}, image.ErrMalformedImageID
// 	}

// 	d, dOk := val["registry"]
// 	t, tOk := val["tag"]

// 	if !dOk {
// 		if tOk {
// 			i = fmt.Sprintf("%s:%s", i, t)
// 		}
// 		ref, err = image.ParseRef(i)
// 		if err != nil {
// 			return image.Ref{}, err
// 		}
// 		return ref, nil
// 	}
// 	if !tOk {
// 		if dOk {
// 			i = fmt.Sprintf("%s/%s", d, i)
// 		}
// 		ref, err = image.ParseRef(i)
// 		if err != nil {
// 			return image.Ref{}, err
// 		}
// 		return ref, nil
// 	}

// 	name := image.Name{Domain: d, Image: i}
// 	return image.Ref{Name: name, Tag: t}, nil
// }

// // processMaybeImageMap processes value of the image parameter, if it exists
// func processMaybeImageMap(value map[string]interface{}) (*image.Ref, error) {
// 	iVal, ok := value["image"]
// 	if !ok {
// 		return nil, nil
// 	}

// 	var ref image.Ref
// 	var err error
// 	switch iVal.(type) {
// 	case string:
// 		val := iVal.(string)
// 		ref, err = processImageString(value, val)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &ref, nil

// 	case map[string]string:
// 		// image:
// 		// 			registry: docker.io   (sometimes missing)
// 		// 			repository: bitnami/mariadb
// 		// 			tag: 10.1.32					(sometimes version)
// 		val := iVal.(map[string]string)

// 		ref, err = processImageMap(val)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &ref, nil
// 	default:
// 		return nil, nil
// 	}
// }

// func createImageRef(domain, imageName, tag string) image.Ref {
// 	return image.Ref{
// 		Name: image.Name{
// 			Domain: domain,
// 			Image:  imageName,
// 		},
// 		Tag: tag,
// 	}
// }
