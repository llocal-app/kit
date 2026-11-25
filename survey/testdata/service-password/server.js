const express = require('express');
const nodemailer = require('nodemailer');

const app = express();
let mail = { host: '', port: 587, user: '', pass: '', to: '' };

app.use(express.json());
app.use(express.static('public'));

app.get('/api/mail-settings', (req, res) => res.json({ ...mail, pass: mail.pass ? '********' : '' }));

app.put('/api/mail-settings', (req, res) => {
  mail = { ...mail, ...req.body };
  res.json({ ok: true });
});

app.post('/api/alerts', async (req, res) => {
  const relay = nodemailer.createTransport({ host: mail.host, port: mail.port, auth: { user: mail.user, pass: mail.pass } });
  await relay.sendMail({ from: mail.user, to: mail.to, subject: `Door ${req.body.door} opened`, text: new Date().toString() });
  res.status(202).end();
});

app.listen(process.env.PORT || 3000);
