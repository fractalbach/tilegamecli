

let gamestate = {}
let ws = new Websocket("localhost:8080")

function incoming(message) {
	let j = JSON.Parse(message)
	Object.assign(gamestate, j)
	display()
}

function display(elem) {
	if (elem !== undefined) {
		elem.innerText(JSON.Stringify(gamestate))
	}
}

function call(name, ...args) {
	ws.Send(JSON.Stringify({Name: name, Params: args}))
}


