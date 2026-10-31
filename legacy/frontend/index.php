<?php $baseURL = "http://" . $_SERVER['SERVER_NAME'] . ":8080"; ?>

<!DOCTYPE html>
<html lang="en">

<head>
    <link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/css/bootstrap.min.css" integrity="sha384-MCw98/SFnGE8fJT3GXwEOngsV7Zt27NXFoaoApmYm81iuXoPkFOJwJ8ERdknLPMO" crossorigin="anonymous">
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script src="https://code.jquery.com/jquery-3.6.0.js" integrity="sha256-H+K7U5CnXl1h5ywQfKtSj8PCmoN9aaq30gDh27Xc0jk=" crossorigin="anonymous"></script>
    <script type="text/javascript" src="<?php echo $baseURL; ?>/frontend/js/scripts.js"></script>
    <link rel="stylesheet" type="text/css" href="<?php echo $baseURL; ?>/frontend/css/styles.css" />

    <title>Web Scraper</title>

</head>

<body>
    <div class="container bg-t">
        <table class="table table-dark">
            <thea class="thead-dark">
                <tr>
                    <th scope="col">#</th>
                    <th scope="col">Time</th>
                    <th scope="col">PTS</th>
                    <th scope="col">J</th>
                    <th scope="col">V</th>
                    <th scope="col">E</th>
                    <th scope="col">D</th>
                    <th scope="col">GP</th>
                    <th scope="col">GC</th>
                    <th scope="col">SG</th>
                    <th scope="col">CA</th>
                    <th scope="col">CV</th>
                    <th scope="col">%</th>
                </tr>
                </thead>
                <tbody id="quote-list">

                </tbody>
        </table>
    </div>
</body>

</html>