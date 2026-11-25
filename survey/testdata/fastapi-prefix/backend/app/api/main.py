from fastapi import APIRouter

from app.api.routes import items, login
from app.core.config import settings

from .routes import utils

api_router = APIRouter()
api_router.include_router(login.router)
api_router.include_router(items.router)
api_router.include_router(utils.router, prefix=f"{settings.UTILS_STR}")
