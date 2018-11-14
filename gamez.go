package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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

const prefixGeneratedHelp = `
╔══════╦═════════════════════════════════════════════════════════════╗
║ Tile ║                                                             ║
║ Game ║               List of Implemented Functions                 ║
║ Core ║                                                             ║
╚══════╩═════════════════════════════════════════════════════════════╝
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
	GridSize       = 30 // number of tiles for both x and y dimensions
)

var (
	uid               = 1
	gamegrid          = [GridSize][GridSize]int{}
	ents              = NewEntMap()
	center            = &commander.Center{FuncMap: fMap}
	actualHelpMessage = ""
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
//			    Game Commands
// ======================================================================

var fMap = map[string]interface{}{
	"grid":  Grid,
	"login": Login,
	"help":  Help,
	"add":   Add,
	"mult":  Mult,
	"addi":  addi,
}

func Help() string {
	return actualHelpMessage
}

func Grid() string {
	s := ""
	for range gamegrid {
		for range gamegrid {
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

func addi(a, b int) int {
	return a + b
}

// ______________________________________________________________________
// 			 Server Related Stuff
// ======================================================================

//go:generate go run src/file2string.go -src "src/index.html" -dst "page.go" -var "page" -pkg "main"
//go:generate go fmt page.go

var clientNotThere = "MessageWatcher: got a message from id(%v), but no client exists.\n"

func messageWatcher(room *wshandle.ClientRoom) {
	for {
		select {
		case msg := <-room.Messages:
			client, ok := room.Client(msg.Id)
			if !ok {
				log.Printf(clientNotThere, msg.Id)
				continue
			}
			result, err := callParse(string(msg.Data))
			if err != nil {
				fmt.Fprintln(client, err)
			} else {
				fmt.Fprintln(client, result)
			}
		}
	}
}

func callParse(s string) (string, error) {
	if s == "help" {
		return Help(), nil
	}
	result, err := center.CallWithFunctionString(s)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(result), nil
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
		result, err := callParse(line)
		if err != nil {
			fmt.Println(err)
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
	actualHelpMessage = prefixGeneratedHelp + center.HelpMessage()
}

func main() {
	flag.Parse()
	if serverFlag {
		runServer()
	} else {
		runStdin()
	}
}
