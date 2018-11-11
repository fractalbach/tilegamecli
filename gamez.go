package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/tilegame/gameserver/commander"
	"github.com/tilegame/gameserver/wshandle"
)

const helpMessage = `
╔══════╦═════════════════════════════════════════════════════════════╗
║ Tile ║                                                             ║
║ Game ║             Guide to the Game Core Functions                ║
║ Core ║                                                             ║
╚══════╩═════════════════════════════════════════════════════════════╝

About this Guide:
	
	Input is in the form of a text stream.
	1 line corresponds to 1 function.
	The text is interpretted and type checked.
	
	Primative Types are defined based on their 
	corresponding JSON definitions.


List of Functions:
	
	addEnt      (entType)     -> (uid)
	delEnt      (uid)         -> (ok)
	setLocation (uid, x, y)   -> (ok)
	setTarget   (uid, x, y)   -> (ok)
	nextTick    ()            -> (tickNum)


List of Names and their Types:

	uid      :: int        unique identifier
	entType  :: string     the type of entity.
	propKey  :: string     property key.
	propVal  :: string     property value.
	x        :: int        grid location x.
	y        :: int        grid location y.
	tickNum  :: int        number of ticks since start.
	ok       :: boolean    indicates a successful action.
	
	
Comon Interpretter Errors:

	Syntax Error.
	Parameter Type Error.

	
Common Game Errors:

	Entity Type does not exist.
	UID does not exist.


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
	uid      = 1
	gamegrid = [GridSize][GridSize]int{}
	ents     = NewEntMap()
)

func nextuid() int {
	uid++
	return uid
}

// Location refers to a square on the game grid.
type Location struct {
	X, Y int
}

// Entity is a person or thing that exists in the game grid.  It
// has a location and the ability to move around.  Can be a player,
// monster, or projetile.
type Entity struct {
	Kind    string
	Name    string
	Uid     int
	Pace    int
	Current Location
	Target  Location
	Health  int `json:",omitempty"`
}

// NewPlayer returns a default player character.
func NewPlayer(name string) *Entity {
	return &Entity{
		Kind:   "player",
		Name:   name,
		Uid:    nextuid(),
		Pace:   PacePlayer,
		Health: 100,
	}
}

type EntMap struct {
	mutex sync.Mutex
	data  map[int]Entity
}

func NewEntMap() *EntMap {
	return &EntMap{
		data: map[int]Entity{},
	}
}

func (em *EntMap) Add(e Entity) {
	em.mutex.Lock()
	em.data[e.Uid] = e
	em.mutex.Unlock()
}

// ______________________________________________________________________
//			    Command Center
// ======================================================================

var fMap = map[string]interface{}{
	"grid":  Grid,
	"login": Login,
	"help":  Help,
	"add":   Add,
	"mult":  Mult,
}
var center = &commander.Center{FuncMap: fMap}

// the function names are added onto this in init().
var functionHelpMessage = `List of Implemented Functions:`

// ______________________________________________________________________
//			    Game Commands
// ======================================================================

func Help() string {
	return functionHelpMessage
}

func Grid() string {
	s := ""
	for _ = range gamegrid {
		for _ = range gamegrid {
			s += "."
		}
		s += "\n"
	}
	return s
}

func Login(name string) int {
	return 0
}

func Logout(name string) {
	return
}

func Add(a, b float64) float64 {
	return a + b
}

func Mult(a, b float64) float64 {
	return a * b
}

// ______________________________________________________________________
// 			 Server Related Stuff
// ======================================================================

// default landing page if you arent connecting from the github page.
const page = `
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
<h1> Websocket Splash Page </h1>
<p>Hello there! Send a command with <code>ws.send()</code> in your web console, or enter a command in the input box. </p>
<h2>Command Terminal</h2>
<pre id="log"></pre>
<form id="cmdform">
    	<input id="cmdterm" type="text" placeholder="Type Command" autocomplete="off">
    	<input type="submit" value="Send Command">
</form>
<script>
 var ws = new WebSocket("ws://" + location.host + "/ws");
 var logger = document.querySelector('#log');
 var cmdterm = document.querySelector('#cmdterm');
 var cmdform = document.querySelector('#cmdform');
cmdform.onsubmit = function(event) {
    if (cmdterm.value == "") {
        return false;
    }
    if (!ws) {
        return false;
    }
    ws.send(cmdterm.value);
    cmdterm.value = "";
    cmdterm.focus(); 
    return false;
};
ws.onmessage = function(e) {
    console.log(e.data);
    logger.innerText = logger.innerText + e.data + "\n";
    logger.scrollTop = logger.scrollHeight;
};
 console.log(ws);
</script>
<style>
    #log {
        min-height: 20em;
        height: 20em;
        min-height: 20em;
        resize: vertical;
        background: #eee;
        whitespace: nowrap-pre;
        border: solid 1px gray;
        overflow: auto;
    }
</style>
</body>
</html>
`

func messageWatcher(room *wshandle.ClientRoom) {
	for {
		select {
		case msg := <-room.Messages:
			s, _ := callParse(string(msg.Data))
			log.Println(s)
			if client, ok := room.Client(msg.Id); ok {
				fmt.Fprintln(client, s)
			}
		}
	}
}

func callParse(s string) (string, bool) {
	if s == "help" {
		return Help(), true
	}
	name, args, err := parse(s)
	if err != nil {
		return fmt.Sprint("Parser: ", err), false
	}
	result, err := center.CallWithStrings(name, args)
	if err != nil {
		return fmt.Sprint("Caller: ", err), false
	}
	return fmt.Sprint(result), true
}

// parses strings in the standard function syntax.  FunctionName(arg1, arg2, arg3)
func parse(s string) (string, []string, error) {
	var name string
	var args []string
	var err error
	var currentArgument string

	s = strings.Replace(s, " ", "", -1)

	for i, r := range s {
		switch r {
		case '(':
			s = s[i+1:]
			goto parseArgs
		default:
			name += string(r)
		}
	}
	err = fmt.Errorf("syntax error: '(' not found.")
	goto ret
parseArgs:
	currentArgument = ""
	for i, r := range s {
		switch r {
		case ')':
			s = s[i+1:]
			if len(currentArgument) > 0 {
				args = append(args, currentArgument)
			}
			goto finish
		case ',':
			args = append(args, currentArgument)
			currentArgument = ""
			continue
		default:
			currentArgument += string(r)
		}
	}
	err = fmt.Errorf("syntax error: ')' not found.")
	goto ret
finish:
	if len(s) != 0 {
		err = fmt.Errorf("syntax error: extra characters found after ')'.")
	}
ret:
	return name, args, err
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

func runStdin() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		result, ok := callParse(line)
		if !ok {
			fmt.Println(result)
		} else {
			fmt.Println(result)
		}
	}
}

// ----------------------------------------------------------------------
// Command Line Flags and Main
// ----------------------------------------------------------------------

var (
	serverFlag     = false
	serverFlagHelp = "Run websocket server instead of command line."
)

func init() {
	flag.BoolVar(&serverFlag, "serve", false, serverFlagHelp)
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, helpMessage, "Command Line Usage:\n\n")
		flag.PrintDefaults()
	}

	// generate the active function guide.
	for k, v := range fMap {
		s := fmt.Sprint(reflect.TypeOf(v))
		s = strings.Replace(s, "func", k, 1)
		functionHelpMessage += fmt.Sprint("\n\t", s)
	}
}

func main() {
	flag.Parse()
	if serverFlag {
		runServer()
	} else {
		runStdin()
	}
}
