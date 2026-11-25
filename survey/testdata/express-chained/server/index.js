import express from 'express';
import React from 'react';
import { renderToString } from 'react-dom/server';
import { StaticRouter } from 'react-router-dom/server';
import App from '../client/App';
import orders from './orders';

const app = express();

app.use(express.json());
app.use('/dist', express.static('dist'));
app.use('/', orders);

app.get('*', (req, res) => {
  const html = renderToString(<StaticRouter location={req.url}><App /></StaticRouter>);
  res.send(`<!doctype html><div id="root">${html}</div><script src="/dist/client.js"></script>`);
});

app.listen(process.env.PORT || 3000);
