from fastapi import APIRouter, HTTPException

router = APIRouter(prefix="/items", tags=["items"])

PARTS: dict[str, dict] = {}


@router.get("/")
def list_items():
    return list(PARTS.values())


@router.post("/")
def create_item(item: dict):
    PARTS[item["id"]] = item
    return item


@router.get("/{item_id}")
def read_item(item_id: str):
    if item_id not in PARTS:
        raise HTTPException(status_code=404, detail="Item not found")
    return PARTS[item_id]


@router.delete("/{item_id}")
def delete_item(item_id: str):
    PARTS.pop(item_id, None)
    return {"ok": True}
