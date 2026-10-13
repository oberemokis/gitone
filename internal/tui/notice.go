package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// noticeLifetime это срок жизни информационной заметки в подвале (#209): итог
// pull и push, «Cancelled», подсказка «только для просмотра». Ошибка git и
// вопрос discard остаются до следующего нажатия: их нельзя пропустить,
// отвернувшись на пять секунд.
const noticeLifetime = 5 * time.Second

// noticeExpiredMsg приходит, когда истёк таймер заметки номер id.
type noticeExpiredMsg struct {
	id int
}

func noticeTick(id int) tea.Cmd {
	return tea.Tick(noticeLifetime, func(time.Time) tea.Msg { return noticeExpiredMsg{id: id} })
}

// putNotice ставит заметку в подвал. Каждая заметка получает новый номер, и
// таймер несёт номер своей: истёкший таймер старой заметки не стирает новую,
// какой бы она ни была. fades говорит, что заметка гаснет сама; у пустой
// заметки гаснуть нечему.
func (m Model) putNotice(text string, fades bool) Model {
	m.notice, m.noticeID, m.fades = text, m.noticeID+1, fades && text != ""
	return m
}

// expire стирает заметку, если таймер её собственный. Заметки уже нет, и
// стирать нечего.
func (m Model) expire(msg noticeExpiredMsg) Model {
	if msg.id == m.noticeID {
		m.notice = ""
	}
	return m
}

// wantExpiry заводит таймер, если после сообщения в подвале стоит новая гаснущая
// заметка. Зовётся после каждого сообщения: заметки ставят многие пути, и ни
// один не должен забыть про таймер или тянуть команду наверх до Update. before
// это номер заметки до сообщения.
func (m Model) wantExpiry(before int, cmd tea.Cmd) tea.Cmd {
	if m.noFade || !m.fades || m.noticeID == before {
		return cmd
	}

	return tea.Batch(cmd, noticeTick(m.noticeID))
}
