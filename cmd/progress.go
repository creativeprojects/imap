package cmd

import "github.com/pterm/pterm"

type progresser struct {
	pbar *pterm.ProgressbarPrinter
}

func newProgresser(pbar *pterm.ProgressbarPrinter) *progresser {
	return &progresser{
		pbar: pbar,
	}
}

func (p *progresser) Increment() {
	if p.pbar == nil {
		return
	}
	p.pbar.Increment()
}
