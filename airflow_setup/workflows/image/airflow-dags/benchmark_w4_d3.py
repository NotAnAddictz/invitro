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
    is_paused_upon_creation=False)
def benchmark_w4_d3():
    @task
    @timing
    def extract():
        # dummy data source
        return [10] * 4

    @task
    @timing
    def stage_00(x: int):
        return x + x

    @task
    @timing
    def do_sum(values):
        return sum(values)

    # specify data flow
    intermediate_00 = extract()
    intermediate_01 = stage_00.expand(x=intermediate_00)
    do_sum(intermediate_01)

# execute dag
etl_dag = benchmark_w4_d3()
