package readline

import (
	"container/list"
	"fmt"
)

type hisItem struct {
	Source  []rune
	Version int64
	Tmp     []rune
}

func (h *hisItem) Clean() {
	h.Source = nil
	h.Tmp = nil
}

type HistoryWriter interface {
	Load() ([][]rune, error)
	Append([]rune) error
	Close() error
}

type opHistory struct {
	cfg        *Config
	history    *list.List
	historyVer int64
	current    *list.Element
	writer     HistoryWriter
	enable     bool
}

func newOpHistory(cfg *Config) (o *opHistory) {
	o = &opHistory{
		cfg:     cfg,
		history: list.New(),
		enable:  true,
	}
	return o
}

func (o *opHistory) Reset() {
	o.history = list.New()
	o.current = nil
}

func (o *opHistory) IsHistoryClosed() bool {
	return (o.writer == nil)
}

func (o *opHistory) Init() (err error) {
	if o.IsHistoryClosed() {
		err = o.initHistory()
	}
	return
}

func (o *opHistory) initHistory() error {

	switch {
	case o.cfg.HistoryWrite != nil:
		o.writer = o.cfg.HistoryWrite
	case o.cfg.HistoryFile == "":
		return nil
	default:
		o.writer = NewHistoryFile(o.cfg.HistoryFile, o.cfg.HistoryLimit)
	}

	lines, err := o.writer.Load()

	if err != nil {
		return err
	}

	for _, line := range lines {
		o.Push(line)
		o.Compact()
	}

	o.historyVer++
	o.Push(nil)

	return nil
}

func (o *opHistory) Compact() {
	for o.history.Len() > o.cfg.HistoryLimit && o.history.Len() > 0 {
		o.history.Remove(o.history.Front())
	}
}

func (o *opHistory) Close() {
	if o.writer != nil {
		o.writer.Close()
		o.writer = nil
	}
}

func (o *opHistory) FindBck(isNewSearch bool, rs []rune, start int) (int, *list.Element) {
	for elem := o.current; elem != nil; elem = elem.Prev() {
		item := o.showItem(elem.Value)
		if isNewSearch {
			start += len(rs)
		}
		if elem == o.current {
			if len(item) >= start {
				item = item[:start]
			}
		}
		idx := runes.IndexAllBckEx(item, rs, o.cfg.HistorySearchFold)
		if idx < 0 {
			continue
		}
		return idx, elem
	}
	return -1, nil
}

func (o *opHistory) FindFwd(isNewSearch bool, rs []rune, start int) (int, *list.Element) {
	for elem := o.current; elem != nil; elem = elem.Next() {
		item := o.showItem(elem.Value)
		if isNewSearch {
			start -= len(rs)
			if start < 0 {
				start = 0
			}
		}
		if elem == o.current {
			if len(item)-1 >= start {
				item = item[start:]
			} else {
				continue
			}
		}
		idx := runes.IndexAllEx(item, rs, o.cfg.HistorySearchFold)
		if idx < 0 {
			continue
		}
		if elem == o.current {
			idx += start
		}
		return idx, elem
	}
	return -1, nil
}

func (o *opHistory) showItem(obj interface{}) []rune {
	item := obj.(*hisItem)
	if item.Version == o.historyVer {
		return item.Tmp
	}
	return item.Source
}

func (o *opHistory) Prev() []rune {
	if o.current == nil {
		return nil
	}
	current := o.current.Prev()
	if current == nil {
		return nil
	}
	o.current = current
	return runes.Copy(o.showItem(current.Value))
}

func (o *opHistory) Next() ([]rune, bool) {
	if o.current == nil {
		return nil, false
	}
	current := o.current.Next()
	if current == nil {
		return nil, false
	}

	o.current = current
	return runes.Copy(o.showItem(current.Value)), true
}

// Disable the current history
func (o *opHistory) Disable() {
	o.enable = false
}

// Enable the current history
func (o *opHistory) Enable() {
	o.enable = true
}

func (o *opHistory) debug() {
	Debug("-------")
	for item := o.history.Front(); item != nil; item = item.Next() {
		Debug(fmt.Sprintf("%+v", item.Value))
	}
}

// save history
func (o *opHistory) New(current []rune) (err error) {

	// history deactivated
	if !o.enable {
		return nil
	}

	current = runes.Copy(current)

	// if just use last command without modify
	// just clean lastest history
	if back := o.history.Back(); back != nil {
		prev := back.Prev()
		if prev != nil {
			if runes.Equal(current, prev.Value.(*hisItem).Source) {
				o.current = o.history.Back()
				o.current.Value.(*hisItem).Clean()
				o.historyVer++
				return nil
			}
		}
	}

	if len(current) == 0 {
		o.current = o.history.Back()
		if o.current != nil {
			o.current.Value.(*hisItem).Clean()
			o.historyVer++
			return nil
		}
	}

	if o.current != o.history.Back() {
		// move history item to current command
		currentItem := o.current.Value.(*hisItem)
		// set current to last item
		o.current = o.history.Back()

		current = runes.Copy(currentItem.Tmp)
	}

	// err only can be a IO error, just report
	err = o.Update(current, true)

	// push a new one to commit current command
	o.historyVer++
	o.Push(nil)
	return
}

func (o *opHistory) Revert() {
	o.historyVer++
	o.current = o.history.Back()
}

func (o *opHistory) Update(s []rune, commit bool) (err error) {
	s = runes.Copy(s)
	if o.current == nil {
		o.Push(s)
		o.Compact()
		return
	}
	r := o.current.Value.(*hisItem)
	r.Version = o.historyVer
	if commit {
		r.Source = s
		if o.writer != nil {
			// just report the error
			err = o.writer.Append(r.Source)
		}
	} else {
		r.Tmp = append(r.Tmp[:0], s...)
	}
	o.current.Value = r
	o.Compact()
	return
}

func (o *opHistory) Push(s []rune) {
	s = runes.Copy(s)
	elem := o.history.PushBack(&hisItem{Source: s})
	o.current = elem
}
