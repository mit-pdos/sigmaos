package malloc

type Allocator interface {
	Alloc(b *[]byte, sz int) // Set slice b to point to an allocated buffer of size sz
}
