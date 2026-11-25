import sqlite3

from flask import Flask, g, request

app = Flask(__name__)


def db():
    if "db" not in g:
        g.db = sqlite3.connect("visitors.db")
        g.db.row_factory = sqlite3.Row
        g.db.execute("CREATE TABLE IF NOT EXISTS visitors (id INTEGER PRIMARY KEY, name TEXT NOT NULL, host TEXT, arrived TEXT)")
    return g.db


@app.get("/api/visitors")
def visitors():
    return [dict(r) for r in db().execute("SELECT * FROM visitors ORDER BY arrived DESC")]


@app.post("/api/visitors")
def sign_in_visitor():
    db().execute("INSERT INTO visitors (name, host, arrived) VALUES (?, ?, datetime('now'))", (request.json["name"], request.json["host"]))
    db().commit()
    return {"ok": True}, 201
