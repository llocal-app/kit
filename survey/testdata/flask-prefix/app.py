from flask import Blueprint, Flask

from config import API_PREFIX
from views import reports
from views.orders import (
    orders,
)

app = Flask(__name__)

status = Blueprint("status", __name__, url_prefix="/status")


@status.route("/ping")
def ping():
    return "ok"


app.register_blueprint(orders, url_prefix=API_PREFIX)
app.register_blueprint(reports.bp)
app.register_blueprint(status, url_prefix="/health")
