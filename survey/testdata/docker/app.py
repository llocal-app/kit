import os

import pdfkit
import psycopg2
import redis
from flask import Flask, Response

app = Flask(__name__)
cache = redis.from_url(os.environ["REDIS_URL"])


def db():
    return psycopg2.connect(os.environ["DATABASE_URL"])


@app.get("/api/invoices")
def invoices():
    with db() as con, con.cursor() as cur:
        cur.execute("SELECT id, customer, total FROM invoices ORDER BY id DESC")
        return [dict(zip(("id", "customer", "total"), r)) for r in cur.fetchall()]


@app.get("/api/invoices/<int:invoice_id>.pdf")
def invoice_pdf(invoice_id):
    pdf = cache.get(f"pdf:{invoice_id}") or pdfkit.from_string(f"<h1>Invoice {invoice_id}</h1>", False)
    cache.set(f"pdf:{invoice_id}", pdf)
    return Response(pdf, mimetype="application/pdf")
