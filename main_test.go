package main

import (
	"fmt"
	"testing"

	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"
)

func Test_extractStats(t *testing.T) {
	tests := []struct {
		input      string
		wantResult stats
	}{
		{`open repository
lock repository
load index files
using parent snapshot 38401629
start scan on [./test/]
start backup on [./test/]
scan finished in 0.797s: 58 files, 97.870 MiB

Files:          56 new,     2 changed,     2 unmodified
Dirs:            0 new,     0 changed,     0 unmodified
Data Blobs:     35 new
Tree Blobs:      1 new
Added to the repo: 169.009 KiB

processed 58 files, 97.870 MiB in 0:00
snapshot c7693989 saved`, stats{
			FilesNew:        56,
			FilesChanged:    2,
			FilesUnmodified: 2,
			FilesProcessed:  58,
			BytesAdded:      173065216,
			BytesProcessed:  102624133120,
		}},
	}
	for ii, tt := range tests {
		t.Run(fmt.Sprint(ii), func(t *testing.T) {
			gotResult, err := extractStats(tt.input)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, tt.wantResult, gotResult)
		})
	}
}

func Test_Save(t *testing.T) {
	stats := stats{
		FilesNew:        56,
		FilesChanged:    2,
		FilesUnmodified: 2,
		FilesProcessed:  58,
		BytesAdded:      173065216,
		BytesProcessed:  102624133120,
	}
	err := stats.Save("./stats.json")
	assert.NoError(t, err)
	assert.FileExists(t, "./stats.json")

}

func Test_Load(t *testing.T) {
	expectedStats := stats{
		FilesNew:        56,
		FilesChanged:    2,
		FilesUnmodified: 2,
		FilesProcessed:  58,
		BytesAdded:      173065216,
		BytesProcessed:  102624133120,
	}

	var actualStats stats
	err := actualStats.Load("./stats.json")
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, actualStats)
}
