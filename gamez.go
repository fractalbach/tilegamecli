package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/fatih/color"
	"github.com/fractalbach/ninjaServer/commander"
	"github.com/fractalbach/ninjaServer/wshandle"
)

const Welcome = `
╔════════════════════════════════════════════════════════════════════╗
║                             Welcome to                             ║
║                         ~ The Tile Game ~                          ║
║                      Command Line Interface!                       ║
╚════════════════════════════════════════════════════════════════════╝
`

// ______________________________________________________________________
//			    Gameplay Stuff
// ======================================================================

const (
	// Pace refers to the number of milliseconds that it takes for
	// an entity to move 1 tile.
	PacePlayer     = 300
	PaceMonster    = 400
	PaceProjectile = 200

	// The number of tiles in each direction.
	GridSize = 30
)

var (
	gamegrid = [GridSize][GridSize]int{}
)

var uid = 1

func nextuid() int {
	uid++
	return uid
}

// Player is the entity representing a user's player character.
type Player struct {
	Name   string
	Uid    int
	Pace   int
	Health int
	Points int
}

// Monster is a non-player character that you a player can battle.
type Monster struct {
	Uid    int
	Health int
}

// NewPlayer returns a default player character.
func NewPlayer(name string) *Player {
	return &Player{
		Name:   name,
		Uid:    nextuid(),
		Pace:   PacePlayer,
		Health: 100,
		Points: 0,
	}
}

// ______________________________________________________________________
//			    Game Commands
// ======================================================================

var center = &commander.Center{FuncMap: FuncMap}
var FuncMap = map[string]interface{}{
	"hello": SayHello,
	"grid":  ShowGrid,
}

func ShowCommands() string {
	s := "List of Commands: \n\n"
	for name, function := range FuncMap {
		s += fmt.Sprintf("%s: %s\n", name, reflect.TypeOf(function))
	}
	return s
}

func ShowGrid() string {
	s := ""
	for _ = range gamegrid {
		for _ = range gamegrid {
			s += "."
		}
		s += "\n"
	}
	return s
}

// ______________________________________________________________________
// 			 Server Related Stuff
// ======================================================================

// default landing page if you arent connecting from the github page.
const page = `
<html><body>
<h1> Websocket Splash Page </h1>
<p>Hello there! Check your console!  Use the <code>ws.send()</code>
command to send a message! </p>
<script>
 var ws = new WebSocket("ws://" + location.host + "/ws");
 ws.onmessage = function(e) {console.log(e.data)};
 console.log(ws);
</script>
</body></html>
`

func messageWatcher(room *wshandle.ClientRoom) {
	for {
		select {
		case msg := <-room.Messages:
			s, _ := callSimple(string(msg.Data))
			log.Println(s)
			if client, ok := room.Client(msg.Id); ok {
				fmt.Fprintln(client, s)
			}
		}
	}
}

func SayHello(name string) string {
	return "hello there, " + name
}

func callSimple(s string) (string, bool) {
	arr := strings.Split(s, " ")

	if len(arr) == 0 {
		fmt.Println("need to enter something.")
	}

	name := arr[0]
	args := []string{}

	if name == "help" {
		return ShowCommands(), true
	}

	if len(arr) > 1 {
		args = arr[1:]
	}

	result, err := center.CallWithStrings(name, args)

	if err != nil {
		return fmt.Sprintln(err), false
	}
	return fmt.Sprintln(result), true

}

func runServer() {
	room := wshandle.NewClientRoom()
	go messageWatcher(room)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	mux.HandleFunc("/ws", room.Handle)
	s := &http.Server{
		Addr:    "localhost:8080",
		Handler: mux,
	}
	fmt.Println("listening and serving on", s.Addr)
	log.Fatal(s.ListenAndServe())
}

func handle(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Method)
	fmt.Fprint(w, page)
}

func prompt() {
	color.Set(color.BgBlue)
	fmt.Print("[game]")
	color.Unset()
	fmt.Print(" ")
	color.Set(color.FgGreen)
	fmt.Print(">>>")
	color.Unset()
	fmt.Print(" ")
}

func printWelcome() {
	color.Set(color.FgHiMagenta)
	fmt.Print(Welcome)
	color.Unset()
}

func runStdin() {
	printWelcome()
	s := bufio.NewScanner(os.Stdin)
	prompt()
	for s.Scan() {
		result, ok := callSimple(s.Text())
		if !ok {
			color.Set(color.FgRed)
			fmt.Println(result)
			color.Unset()
		} else {
			fmt.Println(result)
		}
		prompt()
	}
}

func main() {
	// runServer()
	runStdin()
}
