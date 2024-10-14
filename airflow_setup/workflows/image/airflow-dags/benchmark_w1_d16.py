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
def benchmark_w1_d16():
    @task
    @timing
    def extract():
        # dummy data source
        return 10

    @task
    @timing
    def stage_00(x: int):
        return x + x

    @task
    @timing
    def stage_01(x: int):
        return x + x

    @task
    @timing
    def stage_02(x: int):
        return x + x

    @task
    @timing
    def stage_03(x: int):
        return x + x

    @task
    @timing
    def stage_04(x: int):
        return x + x

    @task
    @timing
    def stage_05(x: int):
        return x + x

    @task
    @timing
    def stage_06(x: int):
        return x + x

    @task
    @timing
    def stage_07(x: int):
        return x + x

    @task
    @timing
    def stage_08(x: int):
        return x + x

    @task
    @timing
    def stage_09(x: int):
        return x + x

    @task
    @timing
    def stage_10(x: int):
        return x + x

    @task
    @timing
    def stage_11(x: int):
        return x + x

    @task
    @timing
    def stage_12(x: int):
        return x + x

    @task
    @timing
    def stage_13(x: int):
        return x + x

    @task
    @timing
    def stage_14(x: int):
        return x + x

    # specify data flow
    intermediate_00 = extract()
    intermediate_01 = stage_00(x=intermediate_00)
    intermediate_02 = stage_01(x=intermediate_01)
    intermediate_03 = stage_02(x=intermediate_02)
    intermediate_04 = stage_03(x=intermediate_03)
    intermediate_05 = stage_04(x=intermediate_04)
    intermediate_06 = stage_05(x=intermediate_05)
    intermediate_07 = stage_06(x=intermediate_06)
    intermediate_08 = stage_07(x=intermediate_07)
    intermediate_09 = stage_08(x=intermediate_08)
    intermediate_10 = stage_09(x=intermediate_09)
    intermediate_11 = stage_10(x=intermediate_10)
    intermediate_12 = stage_11(x=intermediate_11)
    intermediate_13 = stage_12(x=intermediate_12)
    intermediate_14 = stage_13(x=intermediate_13)
    stage_14(x=intermediate_14)

# execute dag
etl_dag = benchmark_w1_d16()
