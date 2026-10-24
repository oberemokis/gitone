package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// noticeLifetime это срок жизни заметки (#209): тост с ошибкой или итогом
// (toast.go) гаснет через пять секунд или от следующего нажатия, что раньше.
// Ошибка гаснет так же, как итог (#371): тост стоит над диффом, и висеть до
// нажатия ему незачем. Вопрос discard не заметка, он ждёт ответа в подвале.
const noticeLifetime = 5 * time.Second

// noticeExpiredMsg приходит, когда истёк таймер заметки номер id.
type noticeExpiredMsg struct {
	id int
}

func (msg noticeExpiredMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.expire(msg), nil }

func noticeTick(id int) tea.Cmd {
	return tea.Tick(noticeLifetime, func(time.Time) tea.Msg { return noticeExpiredMsg{id: id} })
}

// putNotice ставит заметку тостом на место прежней. Каждая заметка получает
// новый номер, и таймер несёт номер своей: истёкший таймер старой заметки не
// стирает новую, какой бы она ни была.
func (m Model) putNotice(text string) Model {
	m.notice, m.noticeID = text, m.noticeID+1
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

// wantExpiry заводит таймер, если после сообщения стоит новая заметка.
// Зовётся после каждого сообщения: заметки ставят многие пути, и ни один не
// должен забыть про таймер или тянуть команду наверх до Update. before это
// номер заметки до сообщения. У пустой заметки гаснуть нечему.
func (m Model) wantExpiry(before int, cmd tea.Cmd) tea.Cmd {
	if m.noFade || m.notice == "" || m.noticeID == before {
		return cmd
	}

	return tea.Batch(cmd, noticeTick(m.noticeID))
}
