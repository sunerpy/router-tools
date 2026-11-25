package backup_test

import (
	"testing"

	"github.com/sunerpy/router-tools/internal/backup"
)

func TestParseMTD(t *testing.T) {
	t.Parallel()

	text := "dev:    size   erasesize  name\n" +
		"mtd0: 00100000 00020000 \"0:BOOT\"\n" +
		"mtd1: 00100000 00020000 \"crash\"\n" +
		"mtd2: 00200000 00020000 \"duplicate\"\n" +
		"mtd3: 00200000 00020000 \"duplicate\"\n"
	partitions, err := backup.ParseMTD(text)
	if err != nil {
		t.Fatalf("ParseMTD() error = %v", err)
	}
	if len(partitions) != 4 || partitions[0].Size != 0x100000 || partitions[0].EraseSize != 0x20000 {
		t.Fatalf("partitions = %+v", partitions)
	}
	if partitions[2].Name != partitions[3].Name || partitions[2].Index == partitions[3].Index {
		t.Fatalf("duplicate name handling = %+v", partitions)
	}
}

func TestParseMTDRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"dev: size erasesize name\n",
		"mtd0: 00000000 00020000 \"zero\"\n",
		"mtd0: 00100000 00000000 \"zero erase\"\n",
		"mtd0: 00100000 \"missing\"\n",
		"mtd0: 100000000000000000000 00020000 \"overflow\"\n",
		"mtd0: 00100000 00020000 \"a\"\nmtd0: 00200000 00020000 \"b\"\n",
		"mtdX: 00100000 00020000 \"bad index\"\n",
	}
	for _, input := range tests {
		if _, err := backup.ParseMTD(input); err == nil {
			t.Fatalf("ParseMTD(%q) error = nil", input)
		}
	}
}
