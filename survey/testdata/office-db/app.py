import os

import pyodbc
import requests
from flask import Flask, request

app = Flask(__name__)

CONN = (
    "DRIVER={ODBC Driver 18 for SQL Server};SERVER=sqlserver01.acme.local;DATABASE=Dispatch;"
    "UID=dispatch_app;PWD=" + os.environ["DISPATCH_DB_PASSWORD"] + ";TrustServerCertificate=yes"
)


@app.get("/api/tickets")
def tickets():
    with pyodbc.connect(CONN) as con:
        rows = con.execute("SELECT TicketID, Customer, Status FROM Tickets WHERE Status <> 'Closed'").fetchall()
    return [{"id": r.TicketID, "customer": r.Customer, "status": r.Status} for r in rows]


@app.post("/api/tickets/<int:ticket_id>/print")
def print_ticket(ticket_id):
    requests.post("http://192.168.1.40/print", json={"ticket": ticket_id, "copies": request.json.get("copies", 1)}, timeout=5)
    return {"ok": True}
