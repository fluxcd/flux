/*

This package implements an image metadata cache given a backing k-v
store.

The interface `Client` stands in the k-v store; `Cache` implements
registry.Registry given a `Client`.

The `Warmer` is for continually refreshing the cache by fetching new
metadata from the original image registries.

*/
package cache
