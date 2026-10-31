<?php
if ($_GET['key'] === "123456") {
    include 'model.php';

    $json_data = json_encode($time_master);

    file_put_contents('api/api.json', $json_data);

    echo "arquivos atualizados";
} else {
    echo "parametros GET incorretos";
}
