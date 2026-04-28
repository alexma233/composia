package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
)

func writeTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if err := writeRow(tw, headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(tw, row); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeRow(w io.Writer, values []string) error {
	for index, value := range values {
		if index > 0 {
			if _, err := fmt.Fprint(w, "\t"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, value); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeKV(w io.Writer, pairs [][2]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, pair := range pairs {
		if _, err := fmt.Fprintf(tw, "%s:\t%s\n", pair[0], pair[1]); err != nil {
			return err
		}
	}
	return tw.Flush()
}
