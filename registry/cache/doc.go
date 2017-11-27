/*
This package implements an image metadata cache given a backing k-v
store.

The interface `Client` stands in for the k-v store (e.g., memcached,
in the subpackage); `Cache` implements registry.Registry given a
`Client`.

The `Warmer` is for continually refreshing the cache by fetching new
metadata from the original image registries.
*/
package cache
