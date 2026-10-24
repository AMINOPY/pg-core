// Chimera tunnel server for Cloudflare Workers (free plan is enough to start).
//
// Deploy: create a Worker, paste this file, add a secret variable KEY
// (a long random string). Your tunnel path is "/<KEY>" on your *.workers.dev
// (or custom) domain. In Chimera: SNI = that domain, IP list = Cloudflare edge IPs.
//
// Protocol (same as internal/engine/native): first WebSocket message is
// atyp(1) addr port(2); the Worker answers one byte (0 = connected), then both
// directions carry the raw byte stream.
import { connect } from 'cloudflare:sockets';

function parseTarget(buf) {
  const b = new Uint8Array(buf);
  let i = 1, host;
  switch (b[0]) {
    case 1:
      if (b.length < 7) return null;
      host = Array.from(b.slice(1, 5)).join('.'); i = 5; break;
    case 4: {
      if (b.length < 19) return null;
      const parts = [];
      for (let k = 0; k < 8; k++) parts.push(((b[1 + k * 2] << 8) | b[2 + k * 2]).toString(16));
      host = parts.join(':'); i = 17; break;
    }
    case 3: {
      const n = b[1];
      if (!n || b.length < 2 + n + 2) return null;
      host = new TextDecoder().decode(b.slice(2, 2 + n)); i = 2 + n; break;
    }
    default: return null;
  }
  const port = (b[i] << 8) | b[i + 1];
  if (!port) return null;
  return { host, port, rest: b.slice(i + 2) };
}

export default {
  async fetch(request, env) {
    if (request.headers.get('Upgrade') !== 'websocket') {
      return new Response('Not found', { status: 404 });
    }
    const key = env.KEY || '';
    const path = new URL(request.url).pathname;
    if (!key || path !== '/' + key) return new Response('Not found', { status: 404 });

    const pair = new WebSocketPair();
    const [client, ws] = Object.values(pair);
    ws.accept();

    let writer = null;
    let socket = null;
    let started = false;
    let chain = Promise.resolve();
    const close = () => {
      try { socket && socket.close(); } catch (_) {}
      try { ws.close(); } catch (_) {}
    };

    const onMessage = async (data) => {
      if (!(data instanceof ArrayBuffer)) data = await new Response(data).arrayBuffer();
      if (!started) {
        started = true;
        const t = parseTarget(data);
        if (!t) { ws.send(new Uint8Array([1])); return close(); }
        try {
          socket = connect({ hostname: t.host, port: t.port });
          await socket.opened;
        } catch (_) { ws.send(new Uint8Array([1])); return close(); }
        ws.send(new Uint8Array([0]));
        writer = socket.writable.getWriter();
        socket.readable
          .pipeTo(new WritableStream({
            write(chunk) { ws.send(chunk); },
            close() { close(); },
            abort() { close(); },
          }))
          .catch(close);
        if (t.rest.length) await writer.write(t.rest);
        return;
      }
      if (writer) await writer.write(new Uint8Array(data));
    };

    ws.addEventListener('message', (ev) => {
      chain = chain.then(() => onMessage(ev.data)).catch(close);
    });
    ws.addEventListener('close', close);
    ws.addEventListener('error', close);

    return new Response(null, { status: 101, webSocket: client });
  },
};
