import os

from openai import OpenAI

from db import Item, session

client = OpenAI(api_key=os.getenv("OPENAI_API_KEY"))


def reorder_low_stock():
    with session() as s:
        low = [i.name for i in s.query(Item).filter(Item.on_hand < 5)]
    if low:
        client.responses.create(model="gpt-5-mini", input=f"Draft a reorder email for: {', '.join(low)}")
