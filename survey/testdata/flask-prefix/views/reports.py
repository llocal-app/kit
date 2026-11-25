from flask import Blueprint, jsonify

bp = Blueprint("reports", __name__, url_prefix="/reports")


@bp.route("/monthly")
def monthly():
    return jsonify({"orders": 0})
