// Package askpass ведёт вопросы git о логине и пароле в окно gitone (#375).
// Терминал занят интерфейсом, поэтому git идёт с GIT_TERMINAL_PROMPT=0, но до
// терминала он спрашивает программу из GIT_ASKPASS. Этой программой служит
// сам gitone: переменная GITONE_ASKPASS с путём к сокету переводит его в
// клиента, который отдаёт вопрос запущенному gitone и печатает ответ для git.
package askpass

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// socketVar несёт клиенту путь к сокету сервера.
const socketVar = "GITONE_ASKPASS"

// ErrCanceled это вопрос, отменённый в окне: git получает отказ.
var ErrCanceled = errors.New("askpass: canceled")

// Env это окружение сетевой команды git, нулевое ничего не задаёт. Тип
// сравним, поэтому git.Repo с ним остаётся сравнимым.
type Env struct {
	program string
	socket  string
}

// Vars это переменные для exec.Cmd.Env, у нулевого Env их нет.
func (e Env) Vars() []string {
	if e == (Env{}) {
		return nil
	}
	return []string{"GIT_ASKPASS=" + e.program, socketVar + "=" + e.socket}
}

// Request это один вопрос git. Ответ уходит один раз: Answer или Cancel.
type Request struct {
	Prompt string
	reply  chan<- string
}

// Answer отдаёт git текст, пустой тоже.
func (r Request) Answer(text string) {
	r.reply <- text
}

// Cancel отказывает git: клиент выходит с ошибкой.
func (r Request) Cancel() {
	close(r.reply)
}

// Server принимает вопросы клиентов, пока его не закроют.
type Server struct {
	env      Env
	dir      string
	listener net.Listener
	requests chan Request
	done     chan struct{}
}

// Listen открывает сокет в своём каталоге с правами 0700: подключиться может
// только тот же пользователь. Клиентом служит текущий исполняемый файл.
func Listen() (*Server, error) {
	program, err := os.Executable()
	if err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "gitone-askpass-")
	if err != nil {
		return nil, err
	}

	socket := filepath.Join(dir, "socket")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}

	s := &Server{
		env:      Env{program: program, socket: socket},
		dir:      dir,
		listener: listener,
		requests: make(chan Request),
		done:     make(chan struct{}),
	}
	go s.serve()
	return s, nil
}

// Env это окружение, которое ведёт вопросы git к серверу. Без сервера оно
// пустое, и git отказывает, как раньше.
func (s *Server) Env() Env {
	if s == nil {
		return Env{}
	}
	return s.env
}

// Requests отдаёт вопросы по одному в порядке прихода.
func (s *Server) Requests() <-chan Request {
	return s.requests
}

// Close закрывает сокет и удаляет его каталог. Клиенты, которые ещё ждут
// ответа, получают отказ.
func (s *Server) Close() error {
	close(s.done)
	return errors.Join(s.listener.Close(), os.RemoveAll(s.dir))
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.answer(conn)
	}
}

// answer отдаёт вопрос клиента в Requests и пишет ему ответ строкой. Отмена
// закрывает соединение без строки.
func (s *Server) answer(conn net.Conn) {
	defer conn.Close()

	prompt, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}

	reply := make(chan string, 1)
	select {
	case s.requests <- Request{Prompt: strings.TrimSuffix(prompt, "\n"), reply: reply}:
	case <-s.done:
		return
	}

	select {
	case text, answered := <-reply:
		if answered {
			// клиент мог умереть вместе с git, сказать об этом некому
			_, _ = io.WriteString(conn, text+"\n")
		}
	case <-s.done:
	}
}

// Reply отвечает git, если он запустил процесс как GIT_ASKPASS: вопрос из
// args уходит серверу, ответ строкой в out. asked false значит, что процесс
// запущен не git и работает как обычно.
func Reply(args []string, out io.Writer) (asked bool, err error) {
	socket := os.Getenv(socketVar)
	if socket == "" {
		return false, nil
	}
	return true, ask(socket, strings.Join(args, " "), out)
}

func ask(socket, prompt string, out io.Writer) error {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	// протокол построчный, а вопрос git и так в одну строку
	if _, err := io.WriteString(conn, strings.ReplaceAll(prompt, "\n", " ")+"\n"); err != nil {
		return err
	}

	answer, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	if len(answer) == 0 {
		return ErrCanceled
	}

	_, err = out.Write(answer)
	return err
}
