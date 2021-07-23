<?php

include 'model.php';
include 'callApi.php';


?>

<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Web Scraper</title>
</head>

<body>
    <?php

    echo "<pre>";
    print_r($time_master);
    echo "</pre>";
    
    ?>
    <script type="text/javascript">
        var json = <?php echo json_encode($time_master, JSON_UNESCAPED_UNICODE); ?>;
        console.log(json);
    </script>
</body>

</html>