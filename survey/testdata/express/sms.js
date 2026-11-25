const twilio = require('twilio');

const client = twilio(process.env.TWILIO_ACCOUNT_SID, process.env.TWILIO_AUTH_TOKEN);

exports.textCrew = (job) =>
  client.messages.create({ from: process.env.TWILIO_FROM, to: job.crew_phone, body: `Today: ${job.title} at ${job.address}` });
