package inject_test

import (
	"testing"

	"github.com/facebookgo/inject"
)

type LevelA struct {
	B *LevelB `inject:""`
	C *LevelC `inject:""`
}

type LevelB struct {
	C *LevelC `inject:""`
	D *LevelD `inject:""`
}

type LevelE struct {
	F *LevelF `inject:""`
}

type LevelC struct {
	F int
}

type LevelD struct {
	C *LevelC `inject:""`
}

type LevelF struct {
	F int
}

func TestLevels(t *testing.T) {
	var a LevelA
	var b LevelB
	var e LevelE

	var g inject.Graph
	err := g.Provide(
		&inject.Object{Value: &a},
		&inject.Object{Value: &b},
		&inject.Object{Value: &e},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}

	levels := g.Levels()
	if len(levels) != 4 {
		t.Fatalf("expecting 4 levels got %d", len(levels))
	}

	if levels[0][0].Value != &a {
		t.Fatal("did not find LevelA")
	}
	if levels[0][1].Value != &e {
		t.Fatal("did not find LevelE")
	}
	if levels[1][0].Value != &b {
		t.Fatal("did not find LevelB")
	}
	if levels[1][1].Value != e.F {
		t.Fatal("did not find LevelF")
	}
	if levels[2][0].Value != b.D {
		t.Fatal("did not find LevelD")
	}
	if levels[3][0].Value != b.C {
		t.Fatal("did not find LevelC")
	}
}
