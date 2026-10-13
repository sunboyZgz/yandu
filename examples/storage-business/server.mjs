import express from 'express';
import session from 'express-session';
import httpProxy from 'http-proxy';
import { timingSafeEqual } from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const require = createRequire(import.meta.url);
const { Configuration, StorageApi } = require('@yandu/storage-server');
const { YANDU_GATEWAY = 'http://127.0.0.1:19091', YANDU_PROJECT_ID, YANDU_PROJECT_KEY, BUSINESS_PASSWORD, SESSION_SECRET } = process.env;
if (!YANDU_PROJECT_ID || !YANDU_PROJECT_KEY || !BUSINESS_PASSWORD || !SESSION_SECRET || SESSION_SECRET.length < 32) throw new Error('Set YANDU_PROJECT_ID, YANDU_PROJECT_KEY, BUSINESS_PASSWORD and SESSION_SECRET (32+ characters) on the server.');
if (!/^[a-z][a-z0-9-]{0,47}$/.test(YANDU_PROJECT_ID)) throw new Error('Invalid project ID');
const app = express();
app.use(session({ secret: SESSION_SECRET, resave: false, saveUninitialized: false, cookie: { httpOnly: true, sameSite: 'strict', secure: false, maxAge: 3600_000 } }));
app.post('/login', express.urlencoded({ extended: false, limit: '4kb' }), (req, res) => {
  if (req.headers.origin && req.headers.origin !== 'http://127.0.0.1:3000') return res.sendStatus(403);
  const expected = Buffer.from(BUSINESS_PASSWORD), actual = Buffer.from(String(req.body.password));
  if (expected.length !== actual.length || !timingSafeEqual(expected, actual)) return res.status(401).send('Invalid business password');
  req.session.regenerate(error => { if (error) return res.sendStatus(500); req.session.user = 'demo'; req.session.save(() => res.redirect('/')); });
});
const authenticated = (req, res, next) => req.session.user ? next() : res.status(401).json({ error: { code: 'BUSINESS_SESSION_REQUIRED', message: 'Sign in to the business application' } });
app.get('/api/capacity', authenticated, async (_req, res) => {
  try { const api = new StorageApi(new Configuration({ basePath: YANDU_GATEWAY, accessToken: YANDU_PROJECT_KEY })); res.json(await api.getCapacity({ projectId: YANDU_PROJECT_ID })); }
  catch { res.status(503).json({ error: { code: 'STORAGE_UNAVAILABLE' } }); }
});
// Control requests use the generated SDK; file bytes continue through the stream proxy.
const api = new StorageApi(new Configuration({basePath:YANDU_GATEWAY,accessToken:YANDU_PROJECT_KEY}));
app.get('/api/media',authenticated,async (_req,res)=>{
 try{res.json(await api.getCapabilities({projectId:YANDU_PROJECT_ID}));}catch{res.status(503).json({error:{code:'STORAGE_UNAVAILABLE',message:'Cannot query media capabilities'}});}
});
app.get('/api/jobs/:jobId',authenticated,async (req,res)=>{
 if(!/^[a-zA-Z0-9-]{1,128}$/.test(req.params.jobId))return res.sendStatus(404);
 try{res.json(await api.getJob({projectId:YANDU_PROJECT_ID,jobId:req.params.jobId}));}catch(error){res.status(error.response?.status||503).json({error:{code:'JOB_UNAVAILABLE',message:'Cannot query this project job'}});}
});
const proxy = httpProxy.createProxyServer({ target: YANDU_GATEWAY, changeOrigin: true, proxyTimeout: 0, timeout: 0 });
proxy.on('proxyReq', (upstream) => { upstream.removeHeader('cookie'); upstream.removeHeader('x-csrf-token'); upstream.setHeader('authorization', `Bearer ${YANDU_PROJECT_KEY}`); });
proxy.on('proxyRes', upstream => { delete upstream.headers['set-cookie']; if (upstream.headers.location) { const location = new URL(upstream.headers.location, YANDU_GATEWAY); upstream.headers.location = location.pathname + location.search; } });
proxy.on('error', (_error, _req, res) => { if (!res.headersSent) res.writeHead(503, { 'Content-Type': 'application/json', 'Retry-After': '5' }); res.end(JSON.stringify({ error: { code: 'DEVICE_OFFLINE', message: 'Storage device offline' } })); });
app.use('/storage', authenticated, (req, res) => {
  if (req.headers.origin && req.headers.origin !== 'http://127.0.0.1:3000') return res.sendStatus(403);
  const prefix = `/storage/v1/projects/${YANDU_PROJECT_ID}/`;
  if (!req.originalUrl.startsWith(prefix) || /[%\\]/.test(req.path) || req.originalUrl.includes('/credentials')) return res.sendStatus(404);
  req.url = req.originalUrl; delete req.headers.authorization; proxy.web(req, res);
});
app.get('/tus.js', (_req, res) => res.sendFile(require.resolve('tus-js-client/dist/tus.js')));
app.get('/project', authenticated, (_req, res) => res.json({ projectId: YANDU_PROJECT_ID }));
app.use(express.static(fileURLToPath(new URL('./public', import.meta.url))));
app.listen(3000, '127.0.0.1', () => console.log('Business example: http://127.0.0.1:3000'));
