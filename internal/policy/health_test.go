package policy

import "testing"

// The caps agree with the figures they were measured from: every per-type cap is the figure seen with the margin, and none is above the overall cap of its table.
func TestHealthCapsAreAboveWhatWasMeasured(t *testing.T) {
	if MaxRepairPieceHealth != 8750 || MaxRepairPlaceableHealth != 25000 {
		t.Fatalf("the overall caps are %v and %v", MaxRepairPieceHealth, MaxRepairPlaceableHealth)
	}
	for ty, seen := range PieceTypeHealthSeen {
		if seen > PieceHealthSeen || PieceHealthCap(ty) > MaxRepairPieceHealth || PieceHealthCap(ty) < seen {
			t.Errorf("%s: seen %v does not fit the caps of the pieces", ty, seen)
		}
	}
	for ty, seen := range PlaceableTypeHealthSeen {
		if seen > PlaceableHealthSeen || PlaceableHealthCap(ty) > MaxRepairPlaceableHealth || PlaceableHealthCap(ty) < seen {
			t.Errorf("%s: seen %v does not fit the caps of the placeables", ty, seen)
		}
	}
	if MaxRepairPieceHealth >= MaxPieceHealth || MaxRepairPlaceableHealth >= MaxPieceHealth {
		t.Fatal("the outer ceiling must be above every cap")
	}
}

func TestACapIsThatOfTheTypeOrTheOverallCapOfTheTable(t *testing.T) {
	for _, c := range []struct {
		got, want float64
	}{
		{PieceHealthCap("Sardaukar_Foundation"), 8750},
		{PieceHealthCap("Sardaukar_Wall_01"), 7000},
		{PieceHealthCap("Sardaukar_Door_Frame_Wide"), 4375},
		{PieceHealthCap("NoSuchPiece"), 8750},
		{PieceHealthCap(""), 8750},
		{PlaceableHealthCap("MediumWaterCistern_Placeable"), 3125},
		{PlaceableHealthCap("NoSuchPlaceable"), 25000},
	} {
		if c.got != c.want {
			t.Errorf("cap %v, want %v", c.got, c.want)
		}
	}
}
