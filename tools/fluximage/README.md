A tool for examining image registry data

    go build ./

### Usage

    ./fluximage [--raw] item

where `item` is an image reference with a tag, e.g.,
`quay.io/weaveworks/flux:1.1.0`.

The output shows the nature of the data stored in the registry (the
type of schema and the digest), and some of the properties deduced
from the schema.
