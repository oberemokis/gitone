package tui_test

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui"
)

// bubble ведёт модель внутри synctest.Test: команды идут в фоне, как у Bubble
// Tea, а время стоит, пока тест не вызовет time.Sleep. Таймер заметки при этом
// настоящий (в options() он выключен), но ждать пять секунд не приходится.
type bubble struct {
	model tea.Model
	msgs  chan tea.Msg
	held  []tea.Cmd
}

// fading открывает репозиторий с включённым таймером заметок.
func fading(t *testing.T, repo string, args ...string) *bubble {
	t.Helper()

	opts := options(repo, layout.Focus())
	opts.Args = args
	opts.KeepNotices = false

	return newBubble(t, opts)
}

func newBubble(t *testing.T, opts tui.Options) *bubble {
	t.Helper()
	// часы пузыря поддельные, настоящий шаг спиннера ничего не стоит, а
	// короткий из options() крутился бы тысячи раз за каждую секунду ожидания
	opts.SpinnerStep = 0
	return &bubble{model: resize(open(t, opts), 120, 20), msgs: make(chan tea.Msg, 64)}
}

// send подаёт сообщение модели и запускает команду, которую она вернула.
func (b *bubble) send(msg tea.Msg) {
	var cmd tea.Cmd
	b.model, cmd = b.model.Update(msg)
	b.run(cmd)
}

// run выполняет команду в своей горутине, части пакета тоже.
func (b *bubble) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}

	go func() {
		msg := cmd()
		if batch, isBatch := msg.(tea.BatchMsg); isBatch {
			for _, part := range batch {
				b.run(part)
			}
			return
		}
		if msg != nil {
			b.msgs <- msg
		}
	}()
}

// deliver доводит до модели всё, что команды уже ответили, и ждёт, пока все
// горутины пузыря не упрутся в таймеры. Часы при этом не идут.
func (b *bubble) deliver() {
	for {
		synctest.Wait()
		select {
		case msg := <-b.msgs:
			b.send(msg)
		default:
			return
		}
	}
}

// press нажимает клавишу и доводит ответы до модели.
func (b *bubble) press(msg tea.KeyPressMsg) {
	b.send(msg)
	b.deliver()
}

// pressSlow нажимает клавишу, которая запускает pull или перечитывание, и
// запускает только спиннер из пакета команд: git «ещё не ответил». Без этого
// ответ приходил бы сразу, а проверять надо индикатор, который идёт дольше пяти
// секунд. Остальные команды ждут в held до release.
func (b *bubble) pressSlow(msg tea.KeyPressMsg) {
	var cmd tea.Cmd
	b.model, cmd = b.model.Update(msg)
	parts := cmd().(tea.BatchMsg)
	b.run(parts[0])
	b.held = parts[1:]
	b.deliver()
}

// release отдаёт ответ git, который держал pressSlow. Цепочка спиннера на
// нём обрывается: пузырь не закончится, пока в нём ждёт таймер.
func (b *bubble) release() {
	for _, cmd := range b.held {
		b.run(cmd)
	}
	b.deliver()
}

// wait двигает часы на d и доводит до модели сработавшие таймеры.
func (b *bubble) wait(d time.Duration) {
	time.Sleep(d)
	b.deliver()
}

func (b *bubble) footer() string {
	return footerOf(b.model)
}

// Case 1: p на ветке без отставания: «Already up to date» стоит ровно пять
// секунд, потом центр подвала пуст без нажатия клавиш.
func TestPullNoteFadesAfterFiveSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, repo := remoteRepo(t)
		app := fading(t, repo)
		idle := app.footer()

		app.press(key('p'))
		if got := app.footer(); !strings.Contains(got, "Already up to date") {
			t.Fatalf("после pull подвал %q, ожидалась заметка", got)
		}

		app.wait(5*time.Second - time.Nanosecond)
		if got := app.footer(); !strings.Contains(got, "Already up to date") {
			t.Errorf("за наносекунду до пяти секунд подвал %q, заметка должна стоять", got)
		}

		app.wait(time.Nanosecond)
		if got := app.footer(); got != idle {
			t.Errorf("через пять секунд подвал %q, ожидался пустой центр %q", got, idle)
		}
	})
}

// Остальные информационные заметки из задачи гаснут так же: итог перемотки и
// push, «Cancelled», «у записи нет файла», «первый коммит не показывается».
func TestEveryInfoNoteFades(t *testing.T) {
	cases := []struct {
		name  string
		note  string
		start func(t *testing.T) *bubble
	}{
		{
			name: "перемотка pull",
			note: "Fast-forwarded",
			start: func(t *testing.T) *bubble {
				origin, repo := remoteRepo(t)
				publishPeer(t, origin, "b.txt", "two\n")
				app := fading(t, repo)
				app.press(key('p'))
				return app
			},
		},
		{
			name: "push",
			note: "Pushed to origin/main",
			start: func(t *testing.T) *bubble {
				_, repo := remoteRepo(t)
				runGit(t, repo, "add", "a.txt")
				runGit(t, repo, "commit", "-q", "-m", "ahead")
				app := fading(t, repo)
				app.press(key('P'))
				return app
			},
		},
		{
			name: "отмена по esc",
			note: "Cancelled",
			start: func(t *testing.T) *bubble {
				origin, repo := remoteRepo(t)
				publishPeer(t, origin, "b.txt", "two\n")
				app := fading(t, repo)
				app.pressSlow(key('p'))
				app.press(escape)
				app.release()
				return app
			},
		},
		{
			name: "у записи нет файла",
			note: "this entry has no file",
			start: func(t *testing.T) *bubble {
				_, repo := remoteRepo(t)
				app := fading(t, repo, "--stat")
				app.press(space)
				return app
			},
		},
		{
			name: "первый коммит не показывается",
			note: "the first commit is not shown",
			start: func(t *testing.T) *bubble {
				repo := newRepo(t)
				write(t, repo, "a.txt", "one\n")
				commitAll(t, repo)
				write(t, repo, "a.txt", "two\n")
				app := fading(t, repo)
				app.press(key('l'))
				app.press(enter)
				return app
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				app := tc.start(t)
				if got := app.footer(); !strings.Contains(got, tc.note) {
					t.Fatalf("подвал %q, ожидалась заметка %q", got, tc.note)
				}

				app.wait(5*time.Second - time.Nanosecond)
				if got := app.footer(); !strings.Contains(got, tc.note) {
					t.Errorf("за наносекунду до пяти секунд подвал %q, заметка должна стоять", got)
				}

				app.wait(time.Nanosecond)
				if got := app.footer(); strings.Contains(got, tc.note) {
					t.Errorf("через пять секунд подвал %q, заметка должна пропасть", got)
				}
			})
		})
	}
}

// Заметка запуска (ошибка config.json) гаснет через пять секунд, как любая
// другая (#391, случай 3).
func TestStartupNoteFades(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := newRepo(t)
		write(t, repo, "a.txt", "one\n")
		commitAll(t, repo)
		opts := options(repo, layout.Focus())
		opts.KeepNotices = false
		opts.Notice = "config.json: broken"
		app := newBubble(t, opts)

		app.run(app.model.Init())
		app.deliver()
		if notice := noticeOf(app.model); notice != "config.json: broken" {
			t.Fatalf("тост при запуске %q, ожидалась ошибка config.json", notice)
		}

		app.wait(5 * time.Second)
		if notice := noticeOf(app.model); notice != "" {
			t.Errorf("через пять секунд тост %q, ожидалось пусто", notice)
		}
	})
}

// Case 2: заметка Б встала на третьей секунде жизни заметки А и живёт свои пять
// секунд. Таймер А на пятой секунде Б не стирает. Тексты у заметок одинаковые:
// таймер узнаёт свою заметку по номеру, а не по тексту.
func TestNewNoteKeepsItsOwnFiveSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, repo := remoteRepo(t)
		app := fading(t, repo, "HEAD~1", "HEAD")
		const note = "this view is read-only"

		app.press(space)
		app.wait(3 * time.Second)
		app.press(key('x'))
		app.wait(2 * time.Second)
		if got := app.footer(); !strings.Contains(got, note) {
			t.Errorf("на пятой секунде подвал %q, заметка Б должна стоять", got)
		}

		app.wait(3*time.Second - time.Nanosecond)
		if got := app.footer(); !strings.Contains(got, note) {
			t.Errorf("за наносекунду до восьмой секунды подвал %q, заметка Б должна стоять", got)
		}

		app.wait(time.Nanosecond)
		if got := app.footer(); strings.Contains(got, note) {
			t.Errorf("на восьмой секунде подвал %q, заметка Б должна пропасть", got)
		}
	})
}

// Case 3: ошибка git гаснет через пять секунд, как итог (#371): тост стоит
// над диффом, и висеть до нажатия ему незачем. Коммит без застейдженных
// правок.
func TestGitErrorFadesLikeNote(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, repo := remoteRepo(t)
		app := fading(t, repo)

		app.press(key('c'))
		app.press(key('x'))
		app.press(enter)
		if got := app.footer(); !strings.Contains(got, "nothing to commit") {
			t.Fatalf("после коммита без правок подвал %q, ожидалась ошибка", got)
		}

		app.wait(5*time.Second - time.Nanosecond)
		if got := app.footer(); !strings.Contains(got, "nothing to commit") {
			t.Errorf("за наносекунду до пяти секунд подвал %q, ошибка должна стоять", got)
		}

		app.wait(time.Nanosecond)
		if got := app.footer(); strings.Contains(got, "nothing to commit") {
			t.Errorf("через пять секунд подвал %q, ошибка должна пропасть", got)
		}
	})
}

// Вопрос об отмене правок стоит в подвале дольше пяти секунд: на вопрос,
// который исчез, нельзя ответить. Отказ «pull is running» гаснет, как любая
// заметка (#371).
func TestQuestionStaysAndRefusalFades(t *testing.T) {
	cases := []struct {
		name  string
		note  string
		stays bool
		start func(t *testing.T) *bubble
	}{
		{
			name: "отказ во время pull",
			note: "pull is running",
			start: func(t *testing.T) *bubble {
				_, repo := remoteRepo(t)
				app := fading(t, repo)
				app.pressSlow(key('p'))
				app.press(key('x'))
				return app
			},
		},
		{
			name:  "вопрос об отмене правок",
			note:  "Discard changes",
			stays: true,
			start: func(t *testing.T) *bubble {
				_, repo := remoteRepo(t)
				app := fading(t, repo)
				app.press(key('x'))
				return app
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				app := tc.start(t)
				if got := app.footer(); !strings.Contains(got, tc.note) {
					t.Fatalf("подвал %q, ожидалась заметка %q", got, tc.note)
				}

				app.wait(10 * time.Second)
				if got := app.footer(); strings.Contains(got, tc.note) != tc.stays {
					t.Errorf("через десять секунд подвал %q: заметка %q стоит %v, ожидалось %v", got, tc.note, !tc.stays, tc.stays)
				}

				app.release()
				app.press(key('n'))
				app.wait(5 * time.Second)
			})
		})
	}
}

// Case 4: клавиша снимает заметку прежним правилом, а таймер, сработавший
// позже, ничего не стирает: ни новую ошибку, ни пустой подвал.
func TestKeyRemovesNoteAndLateTimerErasesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, repo := remoteRepo(t)
		runGit(t, repo, "checkout", "-q", "--detach")
		app := fading(t, repo, "HEAD~1", "HEAD")
		idle := app.footer()
		const refusal = "pull: HEAD is detached, switch to a branch first"

		app.press(space)
		app.wait(2 * time.Second)
		app.press(key('p'))
		if got := app.footer(); strings.Contains(got, "read-only") || !strings.Contains(got, refusal) {
			t.Fatalf("после клавиши подвал %q, ожидалась только ошибка pull", got)
		}

		app.wait(3 * time.Second)
		if got := app.footer(); !strings.Contains(got, refusal) {
			t.Errorf("после таймера прежней заметки подвал %q, ошибка должна стоять", got)
		}

		app.press(key('j'))
		app.press(space)
		app.press(key('j'))
		if got := app.footer(); got != idle {
			t.Fatalf("после клавиш подвал %q, ожидался пустой центр %q", got, idle)
		}
		app.wait(5 * time.Second)
		if got := app.footer(); got != idle {
			t.Errorf("после таймера снятой заметки подвал %q, ожидался пустой центр %q", got, idle)
		}
	})
}

// Case 5: индикатор pull и спиннер загрузки стоят, пока идёт операция, дольше
// пяти секунд: таймер заметки, сработавший посреди них, их не трогает.
func TestIndicatorsOutliveNoteTimer(t *testing.T) {
	cases := []struct {
		name  string
		press tea.KeyPressMsg
		label string
	}{
		{name: "pull", press: key('p'), label: "Pulling…"},
		{name: "перечитывание", press: key('r'), label: "loading"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				_, repo := remoteRepo(t)
				app := fading(t, repo, "HEAD~1", "HEAD")

				app.press(space)
				app.wait(time.Second)
				app.pressSlow(tc.press)
				if got := app.footer(); !strings.Contains(got, tc.label) {
					t.Fatalf("после клавиши подвал %q, ожидался индикатор %q", got, tc.label)
				}

				app.wait(6 * time.Second)
				if got := app.footer(); !strings.Contains(got, tc.label) {
					t.Errorf("на седьмой секунде подвал %q, индикатор %q должен стоять", got, tc.label)
				}

				app.release()
				app.wait(5 * time.Second)
			})
		})
	}
}

// Case 6: перечитывание диффа посреди пяти секунд заметку не продлевает и не
// стирает. Заметка «Already up to date» ставится вместе с перечитыванием после
// pull, а на второй секунде опрос находит правку и обновляет дифф.
func TestRereadMidwayNeitherExtendsNorErasesNote(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, repo := remoteRepo(t)
		ticks := make(chan time.Time)
		opts := options(repo, layout.Focus())
		opts.KeepNotices = false
		opts.Ticks = ticks
		// в split новая строка стоит справа сверху, под тостом
		opts.Config.Diff.Mode = "unified"
		app := newBubble(t, opts)
		app.run(app.model.Init())

		app.press(key('p'))
		app.wait(2 * time.Second)
		write(t, repo, "a.txt", "three\n")
		ticks <- time.Time{}
		app.deliver()
		if got := app.footer(); !strings.Contains(got, "Already up to date") {
			t.Errorf("после опроса подвал %q, заметка должна стоять", got)
		}
		if body := screenText(app.model); !strings.Contains(body, "three") {
			t.Fatalf("опрос не обновил дифф:\n%s", body)
		}

		app.wait(3*time.Second - time.Nanosecond)
		if got := app.footer(); !strings.Contains(got, "Already up to date") {
			t.Errorf("за наносекунду до пяти секунд подвал %q, заметка должна стоять", got)
		}

		app.wait(time.Nanosecond)
		if got := app.footer(); strings.Contains(got, "Already up to date") {
			t.Errorf("на пятой секунде подвал %q, заметка должна пропасть: опрос её продлил", got)
		}

		close(ticks)
		app.deliver()
	})
}
