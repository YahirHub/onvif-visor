package analytics

import "testing"

func TestScoreDetectionFrameFavorsLargeCenteredSharpObject(t *testing.T) {
	flat := make([]byte, InputWidth*InputHeight*3)
	sharp := make([]byte, len(flat))
	for y := 0; y < InputHeight; y++ {
		for x := 0; x < InputWidth; x++ {
			v := byte(0)
			if (x/4+y/4)%2 == 0 {
				v = 255
			}
			i := (y*InputWidth + x) * 3
			sharp[i], sharp[i+1], sharp[i+2] = v, v, v
		}
	}
	poor := Detection{Score: .9, Box: Box{X1: 4, Y1: 4, X2: 64, Y2: 94}}
	good := Detection{Score: .82, Box: Box{X1: 80, Y1: 45, X2: 240, Y2: 285}}
	poorScore := scoreDetectionFrame(flat, poor)
	goodScore := scoreDetectionFrame(sharp, good)
	if goodScore <= poorScore {
		t.Fatalf("good=%f poor=%f; expected large centered sharp detection to win", goodScore, poorScore)
	}
}

func TestBoxSharpnessRejectsInvalidFrame(t *testing.T) {
	if got := boxSharpness([]byte{1, 2, 3}, Box{X1: 0, Y1: 0, X2: 10, Y2: 10}); got != 0 {
		t.Fatalf("sharpness=%f, want 0", got)
	}
}
