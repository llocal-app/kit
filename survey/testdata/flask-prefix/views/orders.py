from flask import Blueprint, jsonify, request

orders = Blueprint("orders", __name__)


@orders.route("/", methods=["GET", "POST"])
def all_orders():
    if request.method == "POST":
        return jsonify(request.get_json()), 201
    return jsonify([])


@orders.route("/<int:order_id>")
def one_order(order_id):
    return jsonify({"id": order_id})
