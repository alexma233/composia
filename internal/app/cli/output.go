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

func writeTerseRows(w io.Writer, rows [][]string) error {
	for _, row := range rows {
		if err := writeTerseRow(w, row); err != nil {
			return err
		}
	}
	return nil
}

func writeTerseRow(w io.Writer, values []string) error {
	for index, value := range values {
		if index > 0 {
			if _, err := fmt.Fprint(w, " "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, terseValue(value)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeTerseKV(w io.Writer, pairs [][2]string) error {
	for _, pair := range pairs {
		if pair[1] == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s=%s\n", pair[0], terseValue(pair[1])); err != nil {
			return err
		}
	}
	return nil
}

func (application *app) writeTable(headers []string, rows [][]string) error {
	if application.isTerseOutput() {
		return writeTerseRows(application.out, rows)
	}
	return writeTable(application.out, headers, rows)
}

func (application *app) writeKV(pairs [][2]string) error {
	if application.isTerseOutput() {
		return writeTerseKV(application.out, pairs)
	}
	return writeKV(application.out, pairs)
}

func (application *app) writeCount(name string, count uint32) error {
	if application.isTerseOutput() {
		return nil
	}
	_, err := fmt.Fprintf(application.out, "%s: %d\n", name, count)
	return err
}

func (application *app) writeCursor(cursor string) error {
	if cursor == "" {
		return nil
	}
	if application.isTerseOutput() {
		_, err := fmt.Fprintf(application.out, "next_cursor=%s\n", terseValue(cursor))
		return err
	}
	_, err := fmt.Fprintf(application.out, "next_cursor: %s\n", cursor)
	return err
}

func terseValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
