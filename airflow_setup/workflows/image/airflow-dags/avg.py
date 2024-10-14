import pendulum
from airflow.decorators import dag, task
import logging
from functools import wraps
from time import time


# by Jonathan Prieto-Cubides https://stackoverflow.com/questions/1622943/timeit-versus-timing-decorator
def timing(f):
    @wraps(f)
    def wrap(*args, **kw):
        ts = time()
        result = f(*args, **kw)
        te = time()
        logging.info('func:%r args:[%r, %r] took: %f sec. Start: %f, End: %f' % (f.__name__, args, kw, te-ts, ts, te))
        return result
    return wrap

@dag(
    schedule_interval=None,
    start_date=pendulum.datetime(2021, 1, 1, tz="UTC"),
    catchup=False,
)
def compute_avg():
    @task
    @timing
    def extract(params=None):
        logging.info(f"params: {params}")
        return params["data"]

    @task
    @timing
    def convert_types(numbers_list):
        return list(map(float, numbers_list))

    @task
    @timing
    def do_avg(data):
        return sum(data)/len(data)

    # specify data flow
    do_avg(convert_types(extract()))

# execute dag
avg_dag = compute_avg()
