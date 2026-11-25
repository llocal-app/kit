from contextlib import asynccontextmanager

from apscheduler.schedulers.background import BackgroundScheduler
from fastapi import FastAPI

from db import Base, engine
from routers import items
from stock import reorder_low_stock

scheduler = BackgroundScheduler()


@asynccontextmanager
async def lifespan(app: FastAPI):
    Base.metadata.create_all(engine)
    scheduler.add_job(reorder_low_stock, "cron", hour=6)
    scheduler.start()
    yield
    scheduler.shutdown()


app = FastAPI(lifespan=lifespan)
app.include_router(items.router)


@app.get("/health")
def health():
    return {"ok": True}
