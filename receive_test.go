package main

import "testing"

func TestBlockSplitter(t *testing.T) {
	b := newBlockSplitter([]byte(`-----BEGIN KEY EXCHANGE MATERIAL-----

eyJpZHB1YiI6IjM4YmVlZDI1ZjAzYmNjMDhkN2E1YTJkNzEwMWUwNWVlNGJiNGYz
ZTU4MGMxYWI2MjQzNmYyN2ViN2ZiZGVkNWMiLCJkaCI6IjQxMWY2MDNmYWEyODE4
ODgxMWU3NDIzZTNjZjgzNzc4M2M2N2FjYjhmYjRiNjNmNDUxZWVkMDgyOGU5YWI0
NDQiLCJkaDEiOiI2Yjg4MjQxYjNhNjViNDIxOWM4YWNhMWNiYjE1OWM2OWIxOWRk
NTFlNTZkNTljNmZmZDgzYWU2OWQyMWYzMDM5In0K
=AC6N
-----END KEY EXCHANGE MATERIAL----------BEGIN GOAX ENCRYPTED MESSAGE-----

/daa6armIWZ226N+WaDCkyIE9XuiNbRJ0tNWPqtnNTcA9M6NxPcmppEB5tRYbOvs
xuldSH8eLU+slSZcuHhWZSHj72tM6dFm2ZvUQmwxVo7xXT8VRJC4fUyjMtW7LwUG
9hQWFD4p22m5PiXfEOWSZFCDhI+pu4Puy88MZvXI1g==
=uSJC
-----END GOAX ENCRYPTED MESSAGE-----`))

	if !b.Scan() {
		t.Fatal("Scanner didn't advance")
	}
	firstBlock := b.Text()
	expectedFirstBlock := `-----BEGIN KEY EXCHANGE MATERIAL-----

eyJpZHB1YiI6IjM4YmVlZDI1ZjAzYmNjMDhkN2E1YTJkNzEwMWUwNWVlNGJiNGYz
ZTU4MGMxYWI2MjQzNmYyN2ViN2ZiZGVkNWMiLCJkaCI6IjQxMWY2MDNmYWEyODE4
ODgxMWU3NDIzZTNjZjgzNzc4M2M2N2FjYjhmYjRiNjNmNDUxZWVkMDgyOGU5YWI0
NDQiLCJkaDEiOiI2Yjg4MjQxYjNhNjViNDIxOWM4YWNhMWNiYjE1OWM2OWIxOWRk
NTFlNTZkNTljNmZmZDgzYWU2OWQyMWYzMDM5In0K
=AC6N
-----END KEY EXCHANGE MATERIAL-----`

	if firstBlock != expectedFirstBlock {
		t.Fatalf("invalid first block, got %v, expected %v", firstBlock, expectedFirstBlock)
	}

	if !b.Scan() {
		t.Fatal("Scanner didn't advance")
	}
	secondBlock := b.Text()
	expectedSecondBlock := `-----BEGIN GOAX ENCRYPTED MESSAGE-----

/daa6armIWZ226N+WaDCkyIE9XuiNbRJ0tNWPqtnNTcA9M6NxPcmppEB5tRYbOvs
xuldSH8eLU+slSZcuHhWZSHj72tM6dFm2ZvUQmwxVo7xXT8VRJC4fUyjMtW7LwUG
9hQWFD4p22m5PiXfEOWSZFCDhI+pu4Puy88MZvXI1g==
=uSJC
-----END GOAX ENCRYPTED MESSAGE-----`

	if secondBlock != expectedSecondBlock {
		t.Fatalf("invalid second block, got %v, expected %v", secondBlock, expectedSecondBlock)
	}

	if b.Scan() {
		t.Fatal("Shouldn't advance a third time")
	}
	if err := b.Err(); err != nil {
		t.Fatal("Got an error after all scanning: ", err)
	}
}
