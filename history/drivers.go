package history

import (
	"os"

	_ "github.com/cznic/ql/driver"
)

func init() {
	// ql requires a temp directory, but will apparently not create it
	// if it doesn't exist; and that can be the case when run inside a
	// container.
	os.Mkdir(os.TempDir(), 0777)
}
