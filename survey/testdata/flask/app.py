import os

from flask import Flask, redirect, render_template, request, url_for
from flask_login import LoginManager, UserMixin, current_user, login_required, login_user, logout_user
from flask_sqlalchemy import SQLAlchemy
from sendgrid import SendGridAPIClient
from sendgrid.helpers.mail import Mail
from werkzeug.security import check_password_hash

app = Flask(__name__)
app.config["SQLALCHEMY_DATABASE_URI"] = os.environ["DATABASE_URL"]
app.config["SECRET_KEY"] = os.environ["SECRET_KEY"]
db = SQLAlchemy(app)
login_manager = LoginManager(app)
login_manager.login_view = "login"


class User(UserMixin, db.Model):
    id = db.Column(db.Integer, primary_key=True)
    email = db.Column(db.String(120), unique=True, nullable=False)
    password_hash = db.Column(db.String(256), nullable=False)


class WorkOrder(db.Model):
    __tablename__ = "work_orders"
    id = db.Column(db.Integer, primary_key=True)
    title = db.Column(db.String(200), nullable=False)
    status = db.Column(db.String(20), default="open")
    photo = db.Column(db.String(200))
    owner_id = db.Column(db.Integer, db.ForeignKey("user.id"))


@login_manager.user_loader
def load_user(user_id):
    return db.session.get(User, int(user_id))


@app.route("/")
@login_required
def index():
    return render_template("index.html", orders=WorkOrder.query.filter_by(owner_id=current_user.id).all())


@app.route("/login", methods=["GET", "POST"])
def login():
    if request.method == "POST":
        user = User.query.filter_by(email=request.form["email"]).first()
        if user and check_password_hash(user.password_hash, request.form["password"]):
            login_user(user)
            return redirect(url_for("index"))
    return render_template("login.html")


@app.route("/logout")
def logout():
    logout_user()
    return redirect(url_for("login"))


@app.route("/orders/<int:order_id>/photo", methods=["POST"])
@login_required
def add_photo(order_id):
    f = request.files["photo"]
    f.save(os.path.join("uploads", f"{order_id}.jpg"))
    order = db.session.get(WorkOrder, order_id)
    order.photo = f"{order_id}.jpg"
    db.session.commit()
    SendGridAPIClient(os.environ["SENDGRID_API_KEY"]).send(
        Mail(from_email="shop@example.org", to_emails="office@example.org", subject="New photo", plain_text_content=order.title)
    )
    return redirect(url_for("index"))


@app.get("/api/orders")
@login_required
def api_orders():
    return [{"id": o.id, "title": o.title} for o in WorkOrder.query.all()]


if __name__ == "__main__":
    app.run(debug=True, port=5000)
