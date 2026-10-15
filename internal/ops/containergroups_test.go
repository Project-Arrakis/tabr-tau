package ops

import (
	"slices"
	"testing"
)

// ARCH-11: the Live Map's storage types were containerGroups[0], right only while "storage" stayed the first group.
func TestStorageTypesAreFoundByGroupIDNotPosition(t *testing.T) {
	if storageTypes["genericcontainer_placeable"] != "Chest" || storageTypes["smallorerefinery_placeable"] != "" {
		t.Fatalf("storageTypes is not the storage group: %v", storageTypes)
	}
	saved := slices.Clone(containerGroups)
	t.Cleanup(func() { containerGroups = saved })
	slices.Reverse(containerGroups) // "storage" is now last
	got := containerGroupTypes("storage")
	if got["storagecontainer_placeable"] != "Storage Container" || got["spicerefinery_placeable"] != "" {
		t.Fatalf("after reordering the groups, storage is still the storage group: %v", got)
	}
	if refining := containerGroupTypes("refining"); refining["spicerefinery_placeable"] == "" {
		t.Fatalf("refining: %v", refining)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("an unknown group id is a programming error and must not silently return the first group")
		}
	}()
	containerGroupTypes("nonsense")
}
