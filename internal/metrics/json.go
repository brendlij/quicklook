package metrics

import "fmt"

func fmtLoad(l Load) string {
	return fmt.Sprintf(`{"one":%g,"five":%g,"fifteen":%g}`, l.One, l.Five, l.Fifteen)
}

func fmtDisk(d DiskIO) string {
	return fmt.Sprintf(`{"read_bps":%g,"write_bps":%g}`, d.ReadBPS, d.WriteBPS)
}
