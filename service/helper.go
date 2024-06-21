package service

// sliceUint64ToInterface converts slice of uint64 to slice of interface
func sliceUint64ToInterface(items []uint64) []interface{} {
	iface := make([]interface{}, len(items))
	for i := range items {
		iface[i] = items[i]
	}
	return iface
}
