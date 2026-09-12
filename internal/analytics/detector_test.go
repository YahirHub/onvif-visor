//go:build cgo

package analytics

import "testing"

func TestNMS(t *testing.T) {
	boxes := []Box{{X1: 0, Y1: 0, X2: 100, Y2: 100, Score: .9}, {X1: 5, Y1: 5, X2: 95, Y2: 95, Score: .8}, {X1: 200, Y1: 200, X2: 250, Y2: 250, Score: .7}}
	got := nms(boxes, .5)
	if len(got) != 2 || got[0].Score != .9 || got[1].Score != .7 {
		t.Fatalf("nms=%#v", got)
	}
}

func TestDecodePersonBoxesUsesPersonClass(t *testing.T) {
	pred := make([]float32, nanoPoints*nanoChannels)
	pred[0] = .75
	for side := 0; side < 4; side++ {
		pred[nanoClasses+side*(nanoRegMax+1)+2] = 10
	}
	boxes := decodePersonBoxes(pred, .35)
	if len(boxes) != 1 || boxes[0].Score != .75 {
		t.Fatalf("boxes=%#v", boxes)
	}
}
