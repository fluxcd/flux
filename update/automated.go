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
	fmt.Println("\n\t++++++++++++++++ automated.CalculateRelease")
	prefilters := []ControllerFilter{
		&IncludeFilter{a.serviceIDs()},
	}

	fmt.Printf("\t\t+++ prefilters: %v \n", prefilters)

	result := Result{}
	updates, err := rc.SelectServices(result, prefilters, nil)
	fmt.Printf("\t\t\t\t+++ after SelectServices: +++ error:%v \n", err)
	for _, u := range updates {
		fmt.Printf("\t\t\t1 ControllerUpdate(after SelectServices): %#v\n", *u)
		fmt.Printf("\t\t\t2 ControllerUpdate.Update(after SelectServices): %#v\n", u.Updates)
	}
	if err != nil {
		return nil, nil, err
	}

	a.markSkipped(result)
	updates, err = a.calculateImageUpdates(rc, updates, result, logger)
	fmt.Printf("\t\t\t\t+++ after calculateImageUpdates: +++ error:%v \n", err)

	if err != nil {
		return nil, nil, err
	}

	for _, u := range updates {
		fmt.Printf("\t\t\t1 ControllerUpdate(after calculateImageUpdates): %#v\n", *u)
		fmt.Printf("\t\t\t2 ControllerUpdate.Update(after calculateImageUpdates): %#v\n", u.Updates)
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
			fmt.Println(" :O  This release will be skipped - NotInRepo")

			results[v] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

func (a *Automated) calculateImageUpdates(rc ReleaseContext, candidates []*ControllerUpdate, result Result, logger log.Logger) ([]*ControllerUpdate, error) {
	fmt.Println("\n [[[ ============ calculateImageUpdates\n")

	fmt.Printf("\tlength of candidate ControllerUpgrades ... %+v\n", len(candidates))
	for _, c := range candidates {
		fmt.Printf("\t\t\tcandidate ControllerUpgrade ... %+v\n", c)
	}

	updates := []*ControllerUpdate{}

	serviceMap := a.serviceMap()
	for _, u := range candidates {
		fmt.Printf("\t\t candidate ResourceID... %+v\n", u.ResourceID)
		fmt.Printf("\t\t candidate Policy ... %+v\n", u.Resource.Policy())
		fmt.Printf("\t\t candidate Containers ... %+v\n", u.Resource.Containers())
		containers := u.Resource.Containers()
		fmt.Printf("\t\t\t how many containers ... %+v\n", len(containers))

		changes := serviceMap[u.ResourceID]
		containerUpdates := []ContainerUpdate{}
		for _, container := range containers {
			fmt.Printf("\t\t\t 1 [[[ container ... %+v\n", container)

			currentImageID := container.Image
			fmt.Printf("\t\t\t 2 [[[ container image ... %+v\n", container.Image)

			for _, change := range changes {
				fmt.Printf("\n\t\t\t\t 3 [[[ change.Container.Name=%s vs container.Name=%s\n", change.Container.Name, container.Name)
				if change.Container.Name != container.Name {
					continue
				}

				// It turns out this isn't a change after all; skip this container
				fmt.Printf("\n\t\t\t\t 4 [[[ change.ImageID.CanonicalRef=%s vs container.Image.CanonicalRe=%s\n", change.ImageID.CanonicalRef(), container.Image.CanonicalRef())

				if change.ImageID.CanonicalRef() == container.Image.CanonicalRef() {
					continue
				}

				// We transplant the tag here, to make sure we keep
				// the format of the image name as it is in the
				// resource (e.g., to avoid canonicalising it)
				newImageID := currentImageID.WithNewTag(change.ImageID.Tag)
				var err error

				fmt.Printf("\t\t\t\t5 [[[ going to UpdateImage: %+v\n\n", newImageID)
				u.ManifestBytes, err = rc.Manifests().UpdateImage(u.ManifestBytes, u.ResourceID, container.Name, newImageID)
				if err != nil {
					return nil, err
				}

				containerUpdates = append(containerUpdates, ContainerUpdate{
					Container: container.Name,
					Current:   currentImageID,
					Target:    newImageID,
				})

				fmt.Printf("\t\t\t\t6 [[[ container update ... %+v\n", containerUpdates)

			}
		}

		if len(containerUpdates) > 0 {
			fmt.Println("\n[[[ There ARE containerUpdates\n")

			u.Updates = containerUpdates
			updates = append(updates, u)
			result[u.ResourceID] = ControllerResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		} else {
			fmt.Println("\n[[[ There are NO containerUpdates\n")

			result[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		}
	}
	fmt.Println("\n============ END of calculateImageUpdates\n")

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
