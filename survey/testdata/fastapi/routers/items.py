import shutil

from fastapi import APIRouter, UploadFile
from pydantic import BaseModel, Field

from db import Item, session

router = APIRouter(prefix="/api/items", tags=["items"])


class ItemIn(BaseModel):
    sku: str
    name: str
    on_hand: int = Field(default=0, ge=0)


@router.get("/")
def list_items():
    with session() as s:
        return s.query(Item).all()


@router.get("/{item_id}")
def get_item(item_id: int):
    with session() as s:
        return s.get(Item, item_id)


@router.post("/")
def add_item(item: ItemIn):
    with session() as s:
        row = Item(**item.model_dump())
        s.add(row)
        s.commit()
        return row


@router.post("/{item_id}/photo")
def add_photo(item_id: int, photo: UploadFile):
    with open(f"photos/{item_id}.jpg", "wb") as out:
        shutil.copyfileobj(photo.file, out)
    return {"ok": True}
