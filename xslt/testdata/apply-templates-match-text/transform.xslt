<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="text"/>

    <xsl:template match="language">
        <xsl:text>Language: </xsl:text>
        <xsl:apply-templates select="name"/>
        <xsl:text>, Creator: </xsl:text>
        <xsl:apply-templates select="creator"/>
        <xsl:text>&#10;</xsl:text>
    </xsl:template>

    <xsl:template match="name | creator">
        <xsl:apply-templates/>
    </xsl:template>

    <xsl:template match="text()">
        <xsl:value-of select="normalize-space(.)"/>
    </xsl:template>

</xsl:stylesheet>