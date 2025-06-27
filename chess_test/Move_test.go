package chess

import (
	"fmt"
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestNewMove(t *testing.T) {
	for i := range 7 {
		for j := range 7 {
			for k := range 7 {
				for l := range 7 {
					scol := chess.COLUMNS[i]
					srow := j + 1
					ecol := chess.COLUMNS[k]
					erow := l + 1
					uci := fmt.Sprintf("%c%d%c%d", scol, srow, ecol, erow)
					m, err := chess.NewMoveUCI(uci)
					if err != nil {
						t.Errorf("Not able to create new move, UCI=%s, error = %s", uci, err.Error())
						continue
					}
					new_uci := m.String()
					if new_uci != uci {
						t.Errorf("UCI's did not match, input = %s, output = %s", uci, new_uci)
					}
				}
			}
		}
	}
}
