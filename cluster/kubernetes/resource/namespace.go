package resource

import (
	"fmt"
)

type Namespace struct {
	baseObject
}

func (ns *Namespace) ResourceID() string {
	return fmt.Sprintf(`%s %s`, ns.Kind, ns.Meta.Name)
}
