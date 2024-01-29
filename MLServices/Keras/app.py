#!/usr/bin/env python
import pickle
import flask
import os

app = flask.Flask(__name__)
port = int(os.getenv("PORT", 9099))
#model = pickle.load(open("model.pkl","rb"))

@app.route('/predict', methods=['POST'])
def predict():
    #row = flask.request.get_json(force=True)['features']
    # row = [[6.2, 3.4, 5.4, 2.3]]
    #prediction = model.predict(row)
    #response = {'prediction': prediction.tolist()}
    response = {'prediction': [1,2,3]}
    return flask.jsonify(response)

@app.route('/upload', methods=['POST'])
def upload():
    data = flask.request.get_json(force=True)
    response = {'status': 'ok', 'data': data}
    return flask.jsonify(response)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=port)
