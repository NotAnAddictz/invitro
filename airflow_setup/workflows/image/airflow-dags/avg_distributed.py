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
    is_paused_upon_creation=False
)
def compute_avg_distributed():
    @task
    @timing
    def extract():
        params = [1, 2, 3, 4]
        logging.info(f"params: {params}")
        return params

    @task
    @timing
    def compute_sum(numbers_list):
        return sum(map(float, numbers_list))

    @task
    @timing
    def compute_count(numbers_list):
        return len(numbers_list)

    @task
    @timing
    def do_avg(total, count):
        if count != 0:
            return total / count
        else:
            return 0

    # specify data flow
    e = extract()
    s = compute_sum(e)
    c = compute_count(e)
    do_avg(s, c)

# execute dag
avg_dag = compute_avg_distributed()
