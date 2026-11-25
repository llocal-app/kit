import sqlite3

import pandas as pd
import plotly.express as px
import streamlit as st
from openai import OpenAI

from helpers import load_jobs

st.set_page_config(page_title="Job costing", layout="wide")

con = sqlite3.connect("costing.db")
con.execute("CREATE TABLE IF NOT EXISTS estimates (id INTEGER PRIMARY KEY, job TEXT, hours REAL, rate REAL)")

uploaded = st.file_uploader("Upload this week's timesheet (CSV)")
jobs = load_jobs(uploaded) if uploaded else pd.read_sql("SELECT * FROM estimates", con)

st.plotly_chart(px.bar(jobs, x="job", y="hours"))

if st.button("Summarise"):
    client = OpenAI(api_key=st.secrets["OPENAI_API_KEY"])
    reply = client.responses.create(model="gpt-5-mini", input="Summarise these hours:\n" + jobs.to_csv())
    st.write(reply.output_text)
