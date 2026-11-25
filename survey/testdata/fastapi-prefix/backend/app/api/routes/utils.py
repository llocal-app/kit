from fastapi import APIRouter

router = APIRouter(tags=["utils"])


@router.get("/health-check/")
def health_check():
    return True
