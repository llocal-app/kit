require('dotenv').config();
const express = require('express');
const session = require('express-session');
const passport = require('passport');
const LocalStrategy = require('passport-local').Strategy;
const bcrypt = require('bcrypt');
const cron = require('node-cron');
const multer = require('multer');
const Stripe = require('stripe');
const { pool } = require('./db');
const jobs = require('./routes/jobs');
const { textCrew } = require('./sms');

const app = express();
const stripe = Stripe(process.env.STRIPE_SECRET_KEY);
const upload = multer({ dest: 'uploads/' });

app.post('/api/webhooks/stripe', express.raw({ type: 'application/json' }), async (req, res) => {
  const event = stripe.webhooks.constructEvent(req.body, req.headers['stripe-signature'], process.env.STRIPE_WEBHOOK_SECRET);
  if (event.type === 'invoice.paid') {
    await pool.query('UPDATE jobs SET paid = true WHERE invoice_id = $1', [event.data.object.id]);
  }
  res.json({ received: true });
});

app.use(express.json());
app.use(express.static('public'));
app.use(session({ secret: process.env.SESSION_SECRET, resave: false, saveUninitialized: false }));
app.use(passport.initialize());
app.use(passport.session());

passport.use(new LocalStrategy({ usernameField: 'email' }, async (email, password, done) => {
  const { rows } = await pool.query('SELECT * FROM users WHERE email = $1', [email]);
  const ok = rows[0] && (await bcrypt.compare(password, rows[0].password_hash));
  done(null, ok ? rows[0] : false);
}));
passport.serializeUser((u, done) => done(null, u.id));
passport.deserializeUser(async (id, done) => done(null, (await pool.query('SELECT * FROM users WHERE id = $1', [id])).rows[0]));

app.post('/login', passport.authenticate('local'), (req, res) => res.json(req.user));
app.use('/api/jobs', jobs);
app.post('/api/photos', upload.single('photo'), (req, res) => res.json({ file: req.file.filename }));

cron.schedule('0 7 * * 1-5', async () => {
  const { rows } = await pool.query("SELECT * FROM jobs WHERE starts_on = CURRENT_DATE");
  for (const job of rows) await textCrew(job);
});

app.listen(3000, () => console.log('Listening on http://localhost:3000'));
