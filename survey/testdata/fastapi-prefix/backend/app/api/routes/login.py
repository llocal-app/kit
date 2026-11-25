from fastapi import APIRouter

router = APIRouter(tags=["login"])


@router.post("/login/access-token")
def login_access_token(username: str, password: str):
    return {"access_token": username, "token_type": "bearer"}
