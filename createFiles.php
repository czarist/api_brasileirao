<?php

if ($_GET['key'] === "123456") {
    include 'model.php';

    $json_data = json_encode($time_master, JSON_UNESCAPED_UNICODE);
    file_put_contents('api/api.json', $json_data);

    file_put_contents('html.html', file_get_contents($link));

    echo "arquivos atualizados";
} else {
    echo "parametros GET incorretos";
}
