import os

from fastapi import FastAPI
from sqlmodel import Session, SQLModel, create_engine, select

from models import Checkout, CheckoutCreate, Tool, ToolCreate, ToolPublic

engine = create_engine(os.environ["DATABASE_URL"])
app = FastAPI()


@app.on_event("startup")
def create_tables():
    SQLModel.metadata.create_all(engine)


@app.get("/api/tools", response_model=list[ToolPublic])
def list_tools():
    with Session(engine) as s:
        return s.exec(select(Tool).where(Tool.retired == False)).all()


@app.post("/api/tools", response_model=ToolPublic)
def add_tool(tool: ToolCreate):
    with Session(engine) as s:
        row = Tool.model_validate(tool)
        s.add(row)
        s.commit()
        s.refresh(row)
        return row


@app.post("/api/tools/{tool_id}/checkouts")
def check_out(tool_id: int, checkout: CheckoutCreate):
    with Session(engine) as s:
        row = Checkout(tool_id=tool_id, **checkout.model_dump())
        s.add(row)
        s.commit()
        return row
