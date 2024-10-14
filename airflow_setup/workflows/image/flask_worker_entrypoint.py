import json
import logging
import os
import pathlib
import time

from flask import Flask
from flask import request
import subprocess

app = Flask(__name__)
logging.getLogger().setLevel(logging.INFO)

class Timer:
    def __init__(self, func_name, annotations):
        self._times = []
        self._timestamp_annotations = []
        self._func_name = func_name
        self._annotations = annotations

    def time(self, annotation: str):
        self._times.append(time.time())
        self._timestamp_annotations.append(annotation)

    def get_log_line(self):
        return f'TIMING: {json.dumps(self.get_timing_info())}'

    def get_timing_info(self):
        timing_info = {"function": self._func_name, "times": self._times, "timestamp_annotations": self._timestamp_annotations}
        timing_info.update(self._annotations)
        return timing_info


    def update_annotations(self, new_annotations):
        self._annotations.update(new_annotations)


@app.route("/run_task_instance", methods=['POST'])
def run_task_instance():
    timer = Timer("flask_run_task_instance", {})
    timer.time("function_entry")
    data = request.json

    # setup xcom input and output
    annotations = data["annotations"]
    timer.update_annotations(annotations)
    base_path = pathlib.Path('/home/airflow') / annotations["dag_id"] / annotations["task_id"] / annotations["run_id"] / str(annotations["map_index"])
    os.makedirs(base_path, exist_ok=True)
    input_path = base_path / "input"
    with open(input_path, "w") as f:
        json.dump(data["xcoms"], f)
    airflow_output_path = f'{base_path}/output'
    try:
        os.remove(airflow_output_path)
    except FileNotFoundError:
        pass

    # execute task
    timer.time("after_input_processing")
    p = subprocess.Popen(data["args"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    timer.time("after_start_subprocess")
    p.wait()
    timer.time("after_finished_subprocess")
    logging.info(f"exitcode: {p.returncode}; stdout: {p.stdout.read()}; stderr: {p.stderr.read()}")

    # retrieve xcom outputs
    try:
        with open(airflow_output_path, 'r') as f:
            xcoms = [json.loads(line) for line in f.readlines()]
    except FileNotFoundError:
        xcoms = []
    timer.time("function_exit")
    timing_info = timer.get_timing_info()
    res = json.dumps({"xcoms": xcoms, "timing_info": timing_info})
    return res

if __name__ == "__main__":
    app.run(host='0.0.0.0', port='50000')
