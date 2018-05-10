package update

import (
	"bytes"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

type Automated struct {
	Changes []Change
}

type Change struct {
	ServiceID flux.ResourceID
	Container resource.Container
	ImageID   image.Ref
}

func (a *Automated) Add(service flux.ResourceID, container resource.Container, image image.Ref) {
	a.Changes = append(a.Changes, Change{service, container, image})
}

func (a *Automated) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ControllerUpdate, Result, error) {
	fmt.Println("\n\t\t++++++++++++++++ automated.CalculateRelease")
	prefilters := []ControllerFilter{
		&IncludeFilter{a.serviceIDs()},
	}

	result := Result{}
	updates, err := rc.SelectServices(result, prefilters, nil)
	fmt.Printf("\t\t\t1 updates(after SelectServices): %#v\n", updates)
	if err != nil {
		return nil, nil, err
	}

	a.markSkipped(result)
	updates, err = a.calculateImageUpdates(rc, updates, result, logger)
	fmt.Printf("\t\t\t2 updates(after calculateImageUpdates): %#v\n", updates)
	if err != nil {
		return nil, nil, err
	}

	fmt.Println("\n\t\t++++++++++++++++ END automated.CalculateRelease\n")
	return updates, result, err
}

func (a *Automated) ReleaseType() ReleaseType {
	return "automated"
}

func (a *Automated) ReleaseKind() ReleaseKind {
	return ReleaseKindExecute
}

func (a *Automated) CommitMessage(result Result) string {
	images := result.ChangedImages()
	buf := &bytes.Buffer{}
	prefix := ""
	switch len(images) {
	case 0: // FIXME(michael): can we get here?
		fmt.Fprintln(buf, "Auto-release (no images)")
	case 1:
		fmt.Fprint(buf, "Auto-release ")
	default:
		fmt.Fprintln(buf, "Auto-release multiple images")
		fmt.Fprintln(buf)
		prefix = " - "
	}
	for _, im := range images {
		fmt.Fprintf(buf, "%s%s\n", prefix, im)
	}
	return buf.String()
}

func (a *Automated) markSkipped(results Result) {
	for _, v := range a.serviceIDs() {
		if _, ok := results[v]; !ok {
			results[v] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

func (a *Automated) calculateImageUpdates(rc ReleaseContext, candidates []*ControllerUpdate, result Result, logger log.Logger) ([]*ControllerUpdate, error) {
	fmt.Println("\t\t============ calculateImageUpdates\n")
	fmt.Printf("\t\t\tcandidates ... %+v\n", candidates)

	updates := []*ControllerUpdate{}

	serviceMap := a.serviceMap()
	for _, u := range candidates {
		containers := u.Resource.Containers()
		changes := serviceMap[u.ResourceID]
		containerUpdates := []ContainerUpdate{}
		for _, container := range containers {
			fmt.Printf("\t\t\t container ... %+v\n", container)

			currentImageID := container.Image
			for _, change := range changes {
				fmt.Printf("\n\t\t\t\t change.Container.Name=%s vs container.Name=%s\n", change.Container.Name, container.Name)
				if change.Container.Name != container.Name {
					continue
				}

				// It turns out this isn't a change after all; skip this container
				fmt.Printf("\n\t\t\t\t change.ImageID.CanonicalRef=%s vs container.Image.CanonicalRe=%s\n", change.ImageID.CanonicalRef(), container.Image.CanonicalRef())

				if change.ImageID.CanonicalRef() == container.Image.CanonicalRef() {
					continue
				}

				// We transplant the tag here, to make sure we keep
				// the format of the image name as it is in the
				// resource (e.g., to avoid canonicalising it)
				newImageID := currentImageID.WithNewTag(change.ImageID.Tag)
				var err error

				fmt.Printf("\t\t>>> release.calculateImageUpdates: %+v\n\n", newImageID)
				u.ManifestBytes, err = rc.Manifests().UpdateImage(u.ManifestBytes, u.ResourceID, container.Name, newImageID)
				if err != nil {
					return nil, err
				}

				containerUpdates = append(containerUpdates, ContainerUpdate{
					Container: container.Name,
					Current:   currentImageID,
					Target:    newImageID,
				})
			}
		}

		if len(containerUpdates) > 0 {
			u.Updates = containerUpdates
			updates = append(updates, u)
			result[u.ResourceID] = ControllerResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		} else {
			result[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		}
	}

	return updates, nil
}

// serviceMap transposes the changes so they can be looked up by ID
func (a *Automated) serviceMap() map[flux.ResourceID][]Change {
	set := map[flux.ResourceID][]Change{}
	for _, change := range a.Changes {
		set[change.ServiceID] = append(set[change.ServiceID], change)
	}
	return set
}

func (a *Automated) serviceIDs() []flux.ResourceID {
	slice := []flux.ResourceID{}
	for service, _ := range a.serviceMap() {
		slice = append(slice, flux.MustParseResourceID(service.String()))
	}
	return slice
}
