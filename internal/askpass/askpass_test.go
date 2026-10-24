package askpass

import (
	"errors"
	"strings"
	"testing"
)

// Клиент получает ответ строкой, как его ждёт git, пустой ответ тоже строка, а
// отмена отказ без строки (#375, случай 5).
func TestReplyReachesClient(t *testing.T) {
	cases := []struct {
		name    string
		respond func(Request)
		out     string
		err     error
	}{
		{name: "ответ", respond: func(r Request) { r.Answer("token") }, out: "token\n"},
		{name: "пустой ответ", respond: func(r Request) { r.Answer("") }, out: "\n"},
		{name: "отмена", respond: Request.Cancel, err: ErrCanceled},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server, err := Listen()
			if err != nil {
				t.Fatalf("Listen: %v", err)
			}
			t.Cleanup(func() { server.Close() })
			t.Setenv(socketVar, server.Env().socket)

			var out strings.Builder
			replied := make(chan error, 1)
			go func() {
				_, err := Reply([]string{"Password for 'https://example.com':"}, &out)
				replied <- err
			}()
			request := <-server.Requests()
			c.respond(request)

			if err := <-replied; !errors.Is(err, c.err) {
				t.Fatalf("Reply: %v, ожидалось %v", err, c.err)
			}
			if request.Prompt != "Password for 'https://example.com':" {
				t.Errorf("вопрос %q", request.Prompt)
			}
			if out.String() != c.out {
				t.Errorf("git получил %q, ожидалось %q", out.String(), c.out)
			}
		})
	}
}

// Без GITONE_ASKPASS процесс запущен не git, и Reply его не трогает.
func TestReplyWithoutSocket(t *testing.T) {
	t.Setenv(socketVar, "")

	asked, err := Reply([]string{"Username:"}, &strings.Builder{})
	if asked || err != nil {
		t.Errorf("Reply: %v, %v, ожидалось false, nil", asked, err)
	}
}
