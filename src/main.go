
package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"fmt"
)

var bind = flag.String("bind", "127.0.0.1:8080", "bind address")
var endpoint = flag.String("endpoint", "127.0.0.1:8080", "WebSocket address")

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, fmt.Sprintf("ws://%s/ws", *endpoint))
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	hub := newHub()
	go hub.run()
	http.HandleFunc("/wssrv", func(w http.ResponseWriter, r *http.Request) {
		serveServerWs(hub, w, r)
	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveClientWs(hub, w, r)
	})
	http.HandleFunc("/", home)
	log.Fatal(http.ListenAndServe(*bind, nil))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<title>WebRTC C++ Native Vulkan</title>
<style>
html,body {
	margin:0px;
	overflow: hidden;
	position: fixed;
	-webkit-user-select: none;
}
</style>
</head>
<body>
<div id="area">
<video id="video" playsinline autoplay muted></video>
</div>
<script>
(() => {
'use strict';

class MouseControl {
	constructor(datachannel) {
		this.datachannel = datachannel;
		this.area = document.getElementById('area');
		this.area.onmousedown = this.mousedown.bind(this);
		this.area.onmousemove = this.mousemove.bind(this);
		this.area.onmouseup = this.mouseup.bind(this);
		this.area.onwheel = this.wheel.bind(this);
		this.tracking = false;
		this.x = 0;
		this.y = 0;
	}

	mousedown(event) {
		this.x = event.clientX;
		this.y = event.clientY;
		this.tracking = true;
	}

	mousemove(event) {
		if (!this.tracking) return;
		const deltaX = event.clientX - this.x;
		const deltaY = event.clientY - this.y;
		this.x = event.clientX;
		this.y = event.clientY;
		const data = ['m',
			deltaX.toString(10),
			deltaY.toString(10)].join(',');
			this.datachannel.send(data);
	}

	mouseup(event) {
		this.tracking = false;
	}

	wheel(event) {
		const data = ['w', event.deltaY.toString(10)].join(',');
    this.datachannel.send(data);
	}
}

class TouchControl {
	constructor(datachannel) {
		this.datachannel = datachannel;
		this.area = document.getElementById('area');
		this.area.addEventListener('touchstart', this.start.bind(this));
		this.area.addEventListener('touchmove', this.move.bind(this));
		this.area.addEventListener('touchend', this.end.bind(this));
		this.tracking = false;
		this.x = 0;
		this.y = 0;
	}

	start(event) {
		const touch = event.touches[0];
		this.x = touch.clientX;
		this.y = touch.clientY;
		this.tracking = true;
	}

	move(event) {
		if (!this.tracking) return;
		const touch = event.touches[0];
		const deltaX = touch.clientX - this.x;
		const deltaY = touch.clientY - this.y;
		this.x = touch.clientX;
		this.y = touch.clientY;
		const data = ['m',
			deltaX.toString(10),
			deltaY.toString(10)].join(',');
			this.datachannel.send(data);
	}

	end(event) {
		this.tracking = false;
	}
}

class Control {
	constructor(datachannel) {
		this.mouse = new MouseControl(datachannel);
		this.touch = new TouchControl(datachannel);
	}
}

class Session {
	constructor(channel, desc) {
		this.channel = channel;
		this.candidates = [];
		this.buffering = true;
		this.connection = new RTCPeerConnection(this.configuration);
		this.connection.onicecandidate = this.ice.bind(this);
		this.connection.ondatachannel = this.datachannel.bind(this);
		this.connection.ontrack = this.track.bind(this);
		// start negotiation
		(async () => {
			await this.negotiation(desc);
		})();
	}

	get configuration() {
		return {
			iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
		};
	}

	async negotiation(desc) {
		await this.connection.setRemoteDescription(desc);
		const answer = await this.connection.createAnswer();
		await this.connection.setLocalDescription(answer);
		const sdp = this.connection.localDescription.sdp;
		this.channel.invoke('answer', sdp);
	}

	ice(event) {
		const candidate = event.candidate;
		if (!candidate) return;
		this.candidates.push(candidate);
		if (!this.buffering && this.candidates.length > 0) {
			const ices = JSON.stringify(this.candidates);
			this.candidates.length = 0;
			this.channel.invoke('candidate', ices);
		}
	}

	receive(candidates) {
		for (const data of candidates) {
			const candidate = new RTCIceCandidate(data);
			this.connection.addIceCandidate(candidate);
		}
		// send back
		this.buffering = false;
		const ices = JSON.stringify(this.candidates);
		this.candidates.length = 0;
		this.channel.invoke('candidate', ices);
	}

	datachannel(event) {
		const channel = event.channel;
		this.control = new Control(channel);
		// notify remote that we are ready
		this.channel.invoke('acquire', '');
	}

	track(event) {
		const stream = event.streams[0];
		const video = document.getElementById('video');
		video.srcObject = stream;
	}
}

class Channel {
	constructor() {
		this.socket = new WebSocket("{{.}}");
		this.socket.onopen = this.open.bind(this);
		this.socket.onmessage = this.message.bind(this);
		this.session = null;
	}

	open(event) {
		console.log('websocket connected. requesting offer...');
		this.invoke('start', '');
	}

	message(event) {
		const data = JSON.parse(event.data);
		if (data.method === 'offer') {
			console.log('got offer. creating session...');
			const type = { type: 'offer', sdp: data.parameter };
			const desc = new RTCSessionDescription(type);
			this.session = new Session(this, desc);
		}
		if (data.method === 'candidate') {
			const candidates = JSON.parse(data.parameter);
			this.session.receive(candidates);
		}
	}

	invoke(method, parameter) {
		const data = { method: method, parameter: parameter };
		const string = JSON.stringify(data);
		this.socket.send(string);
	}
}

const channel = new Channel();

})();
</script>
</body>
</html>
`))
